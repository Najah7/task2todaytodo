import { useState } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { useLocation, useNavigate } from "react-router"
import { getGetTasksQueryKey, getGetProjectsIdTasksQueryKey, getTasksId, patchTasksId, postProjectsIdTasks, postTasks } from "~/api/generated/tasks"
import { getGetProjectsQueryKey, useGetProjectsOptions } from "~/api/generated/projects"
import { useI18n, useLanguage } from "~/features/i18n/hooks"
import { notify } from "~/features/shared/notification"
import TaskForm from "~/features/task/components/TaskForm"
import TaskFormState from "~/features/task/components/TaskForm/TaskFormState"
import { taskCreateFormValues2requests } from "~/features/task/converters/taskCreateFormValues2request"
import { taskFormValues2actionItemSaveOperations, taskFormValues2taskUpdateRequest } from "~/features/task/converters/taskFormValues2updateRequest"
import { saveTaskActionItems } from "~/features/task/saveActionItems"
import { minutesToDurationText, type TaskFormValues } from "~/features/task/components/TaskForm/schema"
import type { TaskFormSaveResult } from "~/features/task/components/TaskForm"
import type { TaskFormDirtyFields } from "~/features/task/converters/taskFormValues2updateRequest"
import { taskErrorMessageKey, taskFieldErrors } from "~/features/task/errors"
import { getAllActiveProjects } from "~/features/task/queries"

const projectOptionsKey = ["task-form", "active-projects"] as const

export default function TaskNewPage() {
  const i18n = useI18n()
  const { language } = useLanguage()
  const navigate = useNavigate()
  const location = useLocation()
  const queryClient = useQueryClient()
  const [createdTask, setCreatedTask] = useState<{ id: string; revision?: number }> ()
  const [needsRevisionRefresh, setNeedsRevisionRefresh] = useState(false)
  const projectsQuery = useQuery({ queryKey: projectOptionsKey, queryFn: getAllActiveProjects, retry: false })
  const optionsQuery = useGetProjectsOptions({ query: { retry: false } })
  const defaultProjectId = new URLSearchParams(location.search).get("project_id") ?? ""
  const returnTo = taskReturnPath(location.state)

  async function submit(values: TaskFormValues, dirtyFields: TaskFormDirtyFields): Promise<TaskFormSaveResult> {
    let task = createdTask
    const result: TaskFormSaveResult = { complete: true, actionItems: [] }
    let savedAnything = false
    let aiSaved = false

    try {
      if (!task) {
        const request = taskCreateFormValues2requests([values])[0]!
        const created = values.projectId
          ? await postProjectsIdTasks(values.projectId, request)
          : await postTasks(request)
        task = { id: created.id ?? "", revision: created.revision }
        if (!task.id) throw new Error("Task create response missing id")
        setCreatedTask(task)
        result.taskSaved = true
        result.taskResetValues = taskFormTaskValues(values, created)
        savedAnything = true
      } else {
        const request = taskFormValues2taskUpdateRequest(values, dirtyFields)
        if (Object.keys(request).length) {
          if (needsRevisionRefresh || task.revision === undefined) {
            const latest = await getTasksId(task.id)
            task = { ...task, revision: latest.task?.revision }
            setCreatedTask(task)
            setNeedsRevisionRefresh(false)
          }
          const updated = await patchTasksId(task.id, request, { headers: { "If-Match": `"${task.revision}"` } })
          task = { ...task, revision: updated.revision }
          setCreatedTask(task)
          result.taskSaved = true
          result.taskResetValues = taskFormTaskValues(values, updated)
          savedAnything = true
        }
      }
    } catch (error) {
      const fieldErrors = taskFieldErrors(error)
      if (fieldErrors) throw { fieldErrors }
      throw error
    }

    const operations = taskFormValues2actionItemSaveOperations(values, dirtyFields)
    const itemResult = await saveTaskActionItems(task.id, operations)
    result.actionItems = itemResult.outcomes
    savedAnything ||= itemResult.hasSavedItems
    aiSaved = itemResult.hasSavedItems
    if (itemResult.outcomes.some((outcome) => outcome.error)) result.complete = false

    if (aiSaved) {
      try {
        const latest = await getTasksId(task.id)
        task = { ...task, revision: latest.task?.revision }
        setCreatedTask(task)
        setNeedsRevisionRefresh(false)
      } catch {
        setNeedsRevisionRefresh(true)
        result.complete = false
        result.revisionRefreshError = true
      }
    }

    if (savedAnything || operations.some((operation) => operation.type === "discard")) {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: getGetTasksQueryKey() }),
        queryClient.invalidateQueries({ queryKey: getGetProjectsQueryKey() }),
        values.projectId ? queryClient.invalidateQueries({ queryKey: getGetProjectsIdTasksQueryKey(values.projectId) }) : Promise.resolve(),
      ])
    }
    if (result.revisionRefreshError) result.complete = false
    return result
  }

  function optionsForForm() {
    const projects = projectsQuery.data ?? []
    const currentProject = defaultProjectId && !projects.some((project) => project.id === defaultProjectId)
      ? [{ value: defaultProjectId, label: defaultProjectId }]
      : []
    const priorityOptions = (optionsQuery.data?.priorities ?? []).map((priority) => ({ value: priority.value, label: language === "ja" ? priority.label_jp : priority.label }))
    return { projectOptions: [...currentProject, ...projects.map((project) => ({ value: project.id, label: project.title }))], priorityOptions }
  }

  if (projectsQuery.isLoading || optionsQuery.isLoading) return <TaskFormState heading="tasks.form.createTitle" message="tasks.loading" status />
  if (!projectsQuery.data || !optionsQuery.data) return <TaskFormState heading="tasks.form.createTitle" message="tasks.options.loadError" retry={() => { void projectsQuery.refetch(); void optionsQuery.refetch() }} />

  const { projectOptions, priorityOptions } = optionsForForm()
  return (
    <TaskForm
      mode="create"
      initialValues={{ title: "", projectId: defaultProjectId, dueDate: "", description: "", priority: "", manualEstimate: "", actionItems: [] }}
      returnTo={returnTo}
      projectOptions={projectOptions}
      priorityOptions={priorityOptions}
      onSubmit={submit}
      onSaveComplete={() => {
        notify.success(i18n("tasks.toast.created"))
        navigate(returnTo, { replace: true })
      }}
      onError={(error) => notify.error(i18n(taskErrorMessageKey(error)))}
    />
  )
}

function taskFormTaskValues(values: TaskFormValues, response: { title?: string; description?: string; due_date?: string | null; priority?: string | { value?: string }; project_id?: string | null; manual_estimated_minutes?: number | null }): NonNullable<TaskFormSaveResult["taskResetValues"]> {
  return {
    title: response.title ?? values.title,
    description: response.description ?? values.description,
    dueDate: response.due_date ?? values.dueDate,
    priority: typeof response.priority === "string" ? response.priority : response.priority?.value ?? values.priority,
    projectId: response.project_id ?? values.projectId,
    manualEstimate: response.manual_estimated_minutes === null ? "" : response.manual_estimated_minutes === undefined ? values.manualEstimate : minutesToDurationText(response.manual_estimated_minutes),
  }
}

function taskReturnPath(state: unknown): string {
  if (state && typeof state === "object" && "from" in state && typeof state.from === "string" && state.from.startsWith("/tasks")) return state.from
  return "/tasks?status=open"
}
