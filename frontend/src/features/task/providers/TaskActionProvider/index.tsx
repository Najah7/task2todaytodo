import { useContext, useEffect, useRef, useState, type ReactNode } from "react"
import { useIsMutating, useMutation, useQueryClient } from "@tanstack/react-query"
import {
  getGetProjectsIdTasksQueryKey,
  getGetTasksIdQueryKey,
  getGetTasksQueryKey,
  getGetTasksTaskIdActionItemsQueryKey,
  getTasksId,
  patchTasksId,
  postTasksTaskIdActionItemsIdcomplete,
  postTasksTaskIdActionItemsIdreopen,
  postProjectsIdTasks,
  postTasks,
} from "~/api/generated/tasks"
import { getGetProjectsQueryKey } from "~/api/generated/projects"
import { useI18n } from "~/features/i18n/hooks"
import { notify } from "~/features/shared/notification"
import { TaskAPIDataContext } from "~/features/task/providers/TaskAPIDataProvider/context"
import {
  TaskActionContext,
  type TaskActions,
  type TaskFormReloadSnapshot,
  type TaskFormSaveResult,
  type TaskFormSubmissionError,
  type UpdateActionItemCompletionInput,
} from "./context"
import { taskCreateFormValues2requests } from "~/features/task/converters/taskCreateFormValues2request"
import { taskAndActionItems2formValues } from "~/features/task/converters/taskAndActionItems2formValues"
import { taskFormValues2actionItemSaveOperations, taskFormValues2taskUpdateRequest } from "~/features/task/converters/taskFormValues2updateRequest"
import { saveTaskActionItems } from "./saveActionItems"
import { minutesToDurationText, type TaskFormValues } from "~/features/task/components/TaskForm/schema"
import type { TaskFormDirtyFields } from "~/features/task/converters/taskFormValues2updateRequest"
import { taskErrorMessageKey, taskFieldErrors } from "~/features/task/errors"
import { getAllTaskActionItems } from "~/features/task/queries"
import { ApiError } from "~/api/http"

const actionItemCompletionMutationKey = ["tasks", "action-item-completion"] as const

type Props = { children: ReactNode }

export function TaskActionProvider({ children }: Props) {
  const data = useContext(TaskAPIDataContext)!
  const queryClient = useQueryClient()
  const i18n = useI18n()
  const [createdTask, setCreatedTask] = useState<{ id: string; revision?: number }>()
  const [needsRevisionRefresh, setNeedsRevisionRefresh] = useState(false)
  const baselineRevision = useRef<number | undefined>(undefined)

  useEffect(() => {
    if (data.taskId && data.state === "ready" && baselineRevision.current === undefined && data.task) {
      baselineRevision.current = data.task.revision
    }
  }, [data.state, data.task, data.taskId])

  const completionMutation = useMutation({
    mutationKey: actionItemCompletionMutationKey,
    mutationFn: ({ taskId, actionItemId, seriesId, occurrenceDate, completed }: UpdateActionItemCompletionInput) => {
      const id = seriesId ?? actionItemId
      const request = seriesId && occurrenceDate ? { occurrence_date: occurrenceDate } : undefined
      return completed
        ? postTasksTaskIdActionItemsIdcomplete(taskId, id, request)
        : postTasksTaskIdActionItemsIdreopen(taskId, id, request)
    },
    onSuccess: async (_response, input) => {
      notify.success(i18n("tasks.toast.actionItemUpdated"))
      await invalidateCompletionQueries(input.taskId)
    },
    onError: async (_error, input) => {
      notify.error(i18n("tasks.form.genericError"))
      await invalidateCompletionQueries(input.taskId)
    },
  })

  async function invalidateCompletionQueries(taskId: string) {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: getGetTasksQueryKey() }),
      queryClient.invalidateQueries({ queryKey: getGetProjectsQueryKey() }),
      queryClient.invalidateQueries({ queryKey: getGetTasksTaskIdActionItemsQueryKey(taskId) }),
    ])
  }

  async function submit(values: TaskFormValues, dirtyFields: TaskFormDirtyFields): Promise<TaskFormSaveResult> {
    return data.taskId
      ? submitEdit(values, dirtyFields)
      : submitCreate(values, dirtyFields)
  }

  async function submitCreate(values: TaskFormValues, dirtyFields: TaskFormDirtyFields): Promise<TaskFormSaveResult> {
    let task = createdTask
    const result: TaskFormSaveResult = { complete: true, actionItems: [] }
    let savedAnything = false
    let actionItemsSaved = false

    if (task && needsRevisionRefresh) {
      try {
        const latest = await getTasksId(task.id)
        if (!latest.task) throw new Error("Task response missing")
        task = { ...task, revision: latest.task.revision }
        setCreatedTask(task)
        setNeedsRevisionRefresh(false)
      } catch {
        result.complete = false
        result.revisionRefreshError = true
        return result
      }
    }

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
          if (task.revision === undefined) {
            const latest = await getTasksId(task.id)
            if (!latest.task) throw new Error("Task response missing")
            task = { ...task, revision: latest.task.revision }
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
    } catch (cause) {
      throwSubmissionFailure(cause, i18n)
    }

    const operations = taskFormValues2actionItemSaveOperations(values, dirtyFields)
    const itemResult = await saveTaskActionItems(task.id, operations)
    result.actionItems = itemResult.outcomes
    savedAnything ||= itemResult.hasSavedItems
    actionItemsSaved = itemResult.hasSavedItems
    if (itemResult.outcomes.some((outcome) => outcome.error)) result.complete = false

    if (actionItemsSaved) {
      try {
        const latest = await getTasksId(task.id)
        if (!latest.task) throw new Error("Task response missing")
        task = { ...task, revision: latest.task.revision }
        setCreatedTask(task)
        setNeedsRevisionRefresh(false)
      } catch {
        setNeedsRevisionRefresh(true)
        result.complete = false
        result.revisionRefreshError = true
      }
    }

    if (savedAnything || operations.some((operation) => operation.type === "discard")) {
      await invalidateTaskQueries(values.projectId)
    }
    return result
  }

  async function submitEdit(values: TaskFormValues, dirtyFields: TaskFormDirtyFields): Promise<TaskFormSaveResult> {
    const task = data.task
    let revision = baselineRevision.current
    if (!task || revision === undefined) return { complete: false }
    const result: TaskFormSaveResult = { complete: true, actionItems: [] }
    let savedAnything = false
    const taskRequest = taskFormValues2taskUpdateRequest(values, dirtyFields)

    if (needsRevisionRefresh) {
      try {
        const latest = await getTasksId(task.id)
        if (!latest.task) throw new Error("Task response missing")
        revision = latest.task.revision
        baselineRevision.current = revision
        queryClient.setQueryData(getGetTasksIdQueryKey(task.id), latest)
        setNeedsRevisionRefresh(false)
      } catch {
        result.complete = false
        result.revisionRefreshError = true
        return result
      }
    }

    if (Object.keys(taskRequest).length) {
      try {
        const updated = await patchTasksId(task.id, taskRequest, { headers: { "If-Match": `"${revision}"` } })
        queryClient.setQueryData(getGetTasksIdQueryKey(task.id), { ...data.taskQuery.data, task: updated })
        baselineRevision.current = updated.revision
        result.taskSaved = true
        result.taskResetValues = taskFormTaskValues(values, updated)
        savedAnything = true
      } catch (cause) {
        throwSubmissionFailure(cause, i18n, true)
      }
    }

    const operations = taskFormValues2actionItemSaveOperations(values, dirtyFields)
    const itemResult = await saveTaskActionItems(task.id, operations)
    result.actionItems = itemResult.outcomes
    savedAnything ||= itemResult.hasSavedItems
    if (itemResult.outcomes.some((outcome) => outcome.error)) result.complete = false

    if (itemResult.hasSavedItems) {
      try {
        const latest = await getTasksId(task.id)
        if (!latest.task) throw new Error("Task response missing")
        queryClient.setQueryData(getGetTasksIdQueryKey(task.id), latest)
        baselineRevision.current = latest.task.revision
        setNeedsRevisionRefresh(false)
      } catch {
        setNeedsRevisionRefresh(true)
        result.complete = false
        result.revisionRefreshError = true
      }
    }

    if (savedAnything || operations.some((operation) => operation.type === "discard")) {
      await invalidateTaskQueries(task.project_id ?? undefined, task.id)
      if (values.projectId && values.projectId !== task.project_id) {
        await queryClient.invalidateQueries({ queryKey: getGetProjectsIdTasksQueryKey(values.projectId) })
      }
    }
    return result
  }

  async function invalidateTaskQueries(projectId?: string, taskId?: string) {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: getGetTasksQueryKey() }),
      queryClient.invalidateQueries({ queryKey: getGetProjectsQueryKey() }),
      ...(taskId ? [
        queryClient.invalidateQueries({ queryKey: getGetTasksIdQueryKey(taskId) }),
        queryClient.invalidateQueries({ queryKey: getGetTasksTaskIdActionItemsQueryKey(taskId) }),
      ] : []),
      ...(projectId ? [queryClient.invalidateQueries({ queryKey: getGetProjectsIdTasksQueryKey(projectId) })] : []),
    ])
  }

  async function retry(): Promise<void> {
    if (data.list) {
      await Promise.all([data.tasksQuery.refetch(), data.projectsQuery.refetch()])
      return
    }
    const queries: Promise<unknown>[] = [data.projectsQuery.refetch(), data.optionsQuery.refetch()]
    if (data.taskId) {
      const taskResult = await data.taskQuery.refetch()
      if (taskResult.data?.task) queries.push(data.actionItemsQuery.refetch())
    }
    await Promise.all(queries)
  }

  async function retryOptions(): Promise<void> {
    if (data.list) {
      await data.projectsQuery.refetch()
      return
    }
    await Promise.all([data.projectsQuery.refetch(), data.optionsQuery.refetch()])
  }

  async function reloadLatest(): Promise<TaskFormReloadSnapshot> {
    const task = data.task
    if (!task || !data.taskId) throw new Error("Task is not loaded")
    try {
      const [latestTaskDetails, latestItems] = await Promise.all([
        getTasksId(task.id),
        getAllTaskActionItems(task.id),
      ])
      const latest = latestTaskDetails.task
      if (!latest) throw new Error("Task response missing")
      if (!latest.can_update) throw new Error("Task is no longer editable")
      const snapshot = { values: taskAndActionItems2formValues([{ task: latest, actionItems: latestItems }])[0]! }
      queryClient.setQueryData(getGetTasksIdQueryKey(task.id), latestTaskDetails)
      queryClient.setQueryData([...getGetTasksTaskIdActionItemsQueryKey(task.id, { page_size: 100 }), "all-pages"], latestItems)
      baselineRevision.current = latest.revision
      setNeedsRevisionRefresh(false)
      return snapshot
    } catch (cause) {
      notify.error(i18n(taskErrorMessageKey(cause)))
      throw cause
    }
  }

  const busy = useIsMutating({ mutationKey: actionItemCompletionMutationKey }) > 0
  const actions: TaskActions = {
    busy,
    submit,
    saveComplete: () => notify.success(i18n(data.taskId ? "tasks.toast.updated" : "tasks.toast.created")),
    retry,
    retryOptions,
    ...(data.taskId ? { reloadLatest } : {}),
    updateActionItemCompletion: (input) => completionMutation.mutateAsync(input).then(() => undefined),
  }

  return <TaskActionContext.Provider value={actions}>{children}</TaskActionContext.Provider>
}

function taskFormTaskValues(values: TaskFormValues, response: {
  title?: string
  description?: string
  due_date?: string | null
  priority?: string | { value?: string }
  project_id?: string | null
  manual_estimated_minutes?: number | null
}): NonNullable<TaskFormSaveResult["taskResetValues"]> {
  return {
    title: response.title ?? values.title,
    description: response.description ?? values.description,
    dueDate: response.due_date ?? values.dueDate,
    priority: typeof response.priority === "string" ? response.priority : response.priority?.value ?? values.priority,
    projectId: response.project_id ?? values.projectId,
    manualEstimate: response.manual_estimated_minutes === null ? "" : response.manual_estimated_minutes === undefined ? values.manualEstimate : minutesToDurationText(response.manual_estimated_minutes),
  }
}

function throwSubmissionFailure(cause: unknown, i18n: ReturnType<typeof useI18n>, detectConflict = false): never {
  if (detectConflict && cause instanceof ApiError && cause.status === 409) {
    const error = new Error("Task revision conflict") as TaskFormSubmissionError
    error.conflict = true
    throw error
  }
  const rawErrors = taskFieldErrors(cause)
  if (rawErrors) {
    const error = new Error("Task validation failed") as TaskFormSubmissionError
    error.fieldErrors = Object.fromEntries(Object.keys(rawErrors).map((field) => [field, "tasks.form.invalidField" as const]))
    throw error
  }
  notify.error(i18n(taskErrorMessageKey(cause)))
  throw cause
}
