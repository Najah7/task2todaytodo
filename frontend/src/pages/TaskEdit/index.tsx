import { useState } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { useLocation, useNavigate, useParams } from "react-router"
import {
  getGetTasksIdQueryKey,
  getGetTasksQueryKey,
  getGetTasksTaskIdActionItemsQueryKey,
  getGetProjectsIdTasksQueryKey,
  getTasksId,
  patchTasksId,
} from "~/api/generated/tasks"
import { getGetProjectsQueryKey, useGetProjectsOptions } from "~/api/generated/projects"
import { useI18n, useLanguage } from "~/features/i18n/hooks"
import { notify } from "~/features/shared/notification"
import TaskForm from "~/features/task/components/TaskForm"
import TaskFormState from "~/features/task/components/TaskForm/TaskFormState"
import { taskAndActionItems2formValues } from "~/features/task/converters/taskAndActionItems2formValues"
import { taskFormValues2actionItemSaveOperations, taskFormValues2taskUpdateRequest } from "~/features/task/converters/taskFormValues2updateRequest"
import { saveTaskActionItems } from "~/features/task/saveActionItems"
import { taskErrorMessageKey, taskFieldErrors } from "~/features/task/errors"
import { getAllActiveProjects, getAllTaskActionItems } from "~/features/task/queries"
import { ApiError } from "~/api/http"
import type { RestActionItemResponse, RestTaskResponse } from "~/api/generated/tasks"
import { minutesToDurationText, type TaskFormValues } from "~/features/task/components/TaskForm/schema"
import type { TaskFormDirtyFields } from "~/features/task/converters/taskFormValues2updateRequest"
import type { TaskFormSaveResult } from "~/features/task/components/TaskForm"

const projectOptionsKey = ["task-form", "active-projects"] as const

export default function TaskEditPage() {
  const { id = "" } = useParams()
  const i18n = useI18n()
  const { language } = useLanguage()
  const navigate = useNavigate()
  const location = useLocation()
  const queryClient = useQueryClient()
  const [hasConflict, setHasConflict] = useState(false)
  const [baselineVersion, setBaselineVersion] = useState(0)
  const taskQuery = useQuery({ queryKey: getGetTasksIdQueryKey(id), queryFn: () => getTasksId(id), enabled: Boolean(id), retry: false })
  const task = taskQuery.data?.task
  const actionItemsQuery = useQuery({
    queryKey: [...getGetTasksTaskIdActionItemsQueryKey(id, { page_size: 100 }), "all-pages"],
    queryFn: () => getAllTaskActionItems(id),
    enabled: Boolean(task),
    retry: false,
  })
  const projectsQuery = useQuery({ queryKey: projectOptionsKey, queryFn: getAllActiveProjects, retry: false })
  const optionsQuery = useGetProjectsOptions({ query: { retry: false } })
  const returnTo = taskReturnPath(location.state)

  async function reloadLatest() {
    const [freshTask, freshItems] = await Promise.all([taskQuery.refetch(), actionItemsQuery.refetch()])
    if (!freshTask.isError && !freshItems.isError && freshTask.data?.task && freshItems.data) {
      setHasConflict(false)
      setBaselineVersion((version) => version + 1)
    }
    else notify.error(i18n("tasks.form.loadError"))
  }

  if (taskQuery.isLoading || projectsQuery.isLoading || optionsQuery.isLoading || (task && actionItemsQuery.isLoading)) {
    return <TaskFormState heading="tasks.form.editTitle" message="tasks.loading" status />
  }
  if (!task || !actionItemsQuery.data || !projectsQuery.data || !optionsQuery.data) {
    return <TaskFormState heading="tasks.form.editTitle" message="tasks.form.loadError" retry={() => { void taskQuery.refetch(); void actionItemsQuery.refetch(); void projectsQuery.refetch(); void optionsQuery.refetch() }} />
  }
  if (!task.can_update) return <TaskFormState heading="tasks.form.editTitle" message="tasks.form.forbidden" />

  const projectOptions = projectsQuery.data.map((project) => ({ value: project.id, label: project.title }))
  if (task.project_id && !projectOptions.some((option) => option.value === task.project_id)) {
    projectOptions.unshift({ value: task.project_id, label: task.project_name ?? task.project_id })
  }
  const priorityOptions = optionsQuery.data.priorities.map((priority) => ({ value: priority.value, label: language === "ja" ? priority.label_jp : priority.label }))

  return (
    <TaskEditForm
      key={`${id}:${baselineVersion}`}
      task={task}
      actionItems={actionItemsQuery.data}
      projectOptions={projectOptions}
      priorityOptions={priorityOptions}
      returnTo={returnTo}
      queryClient={queryClient}
      onConflict={() => setHasConflict(true)}
      onSuccess={() => {
        setHasConflict(false)
        notify.success(i18n("tasks.toast.updated"))
        navigate(returnTo, { replace: true })
      }}
      hasConflict={hasConflict}
      onReloadLatest={reloadLatest}
    />
  )
}

type TaskEditFormProps = {
  task: RestTaskResponse
  actionItems: RestActionItemResponse[]
  projectOptions: { value: string; label: string }[]
  priorityOptions: { value: string; label: string }[]
  returnTo: string
  queryClient: ReturnType<typeof useQueryClient>
  onConflict: () => void
  onSuccess: () => void
  hasConflict: boolean
  onReloadLatest: () => void
}

function TaskEditForm({ task, actionItems, projectOptions, priorityOptions, returnTo, queryClient, onConflict, onSuccess, hasConflict, onReloadLatest }: TaskEditFormProps) {
  const i18n = useI18n()
  const [taskRevision, setTaskRevision] = useState(task.revision)
  const initialValues = taskAndActionItems2formValues([{ task, actionItems }])[0]!

  async function submit(values: TaskFormValues, dirtyFields: TaskFormDirtyFields): Promise<TaskFormSaveResult> {
    const result: TaskFormSaveResult = { complete: true, actionItems: [] }
    let savedAnything = false
    let aiSaved = false
    const taskRequest = taskFormValues2taskUpdateRequest(values, dirtyFields)
    if (Object.keys(taskRequest).length) {
      try {
        const updated = await patchTasksId(task.id, taskRequest, { headers: { "If-Match": `"${taskRevision}"` } })
        setTaskRevision(updated.revision)
        result.taskSaved = true
        result.taskResetValues = taskFormTaskValues(values, updated)
        savedAnything = true
      } catch (error) {
        const fieldErrors = taskFieldErrors(error)
        if (fieldErrors) throw { fieldErrors }
        throw error
      }
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
        if (!latest.task) throw new Error("Task response missing")
        setTaskRevision(latest.task.revision)
      } catch {
        result.complete = false
        result.revisionRefreshError = true
      }
    }

    if (savedAnything || operations.some((operation) => operation.type === "discard")) {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: getGetTasksQueryKey() }),
        queryClient.invalidateQueries({ queryKey: getGetTasksIdQueryKey(task.id) }),
        queryClient.invalidateQueries({ queryKey: getGetTasksTaskIdActionItemsQueryKey(task.id) }),
        queryClient.invalidateQueries({ queryKey: getGetProjectsQueryKey() }),
        task.project_id ? queryClient.invalidateQueries({ queryKey: getGetProjectsIdTasksQueryKey(task.project_id) }) : Promise.resolve(),
        values.projectId && values.projectId !== task.project_id ? queryClient.invalidateQueries({ queryKey: getGetProjectsIdTasksQueryKey(values.projectId) }) : Promise.resolve(),
      ])
    }
    return result
  }

  return (
    <TaskForm
      mode="edit"
      initialValues={initialValues}
      estimateSource={task.estimate_source as "manual" | "action_items"}
      projectOptions={projectOptions}
      priorityOptions={priorityOptions}
      returnTo={returnTo}
      hasConflict={hasConflict}
      onReloadLatest={onReloadLatest}
      onSubmit={submit}
      onSaveComplete={onSuccess}
      onError={(error) => {
        if (error instanceof ApiError && error.status === 409) onConflict()
        else notify.error(i18n(taskErrorMessageKey(error)))
      }}
    />
  )
}

function taskFormTaskValues(values: TaskFormValues, response: RestTaskResponse): NonNullable<TaskFormSaveResult["taskResetValues"]> {
  return {
    title: response.title,
    description: response.description,
    dueDate: response.due_date ?? "",
    priority: response.priority,
    projectId: response.project_id ?? "",
    manualEstimate: response.manual_estimated_minutes === null ? "" : response.manual_estimated_minutes === undefined ? values.manualEstimate : minutesToDurationText(response.manual_estimated_minutes),
  }
}

function taskReturnPath(state: unknown): string {
  if (state && typeof state === "object" && "from" in state && typeof state.from === "string" && state.from.startsWith("/tasks")) return state.from
  return "/tasks"
}
