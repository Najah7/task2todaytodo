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
  postTasksIdcomplete,
  postTasksIdreopen,
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
  type UpdateTaskCompletionInput,
} from "./context"
import { taskCreateFormValues2requests } from "~/features/task/converters/taskCreateFormValues2request"
import { taskAndActionItems2formValues } from "~/features/task/converters/taskAndActionItems2formValues"
import { taskFormValues2actionItemSaveOperations, taskFormValues2taskUpdateRequest } from "~/features/task/converters/taskFormValues2updateRequest"
import { saveTaskActionItems } from "./saveActionItems"
import { minutesToDurationText, type TaskFormValues } from "~/features/task/components/TaskForm/schema"
import type { TaskFormDirtyFields } from "~/features/task/converters/taskFormValues2updateRequest"
import { taskErrorMessageKey, taskFieldErrors } from "~/features/task/errors"
import { getAllTaskActionItems } from "~/features/task/queries"
import { ApiError, isTodoRegenerationRequired } from "~/api/http"
import { getListInboxQueryKey } from "~/api/generated/inbox"
import { getGetTodayTodoQueryKey } from "~/api/generated/todo"
import { addActionItemToToday, addTaskToToday as addTaskToday, isAllDayScheduleBlocksActionItems, makeActionItemUrgent, makeTaskUrgent, showAddedToTodayToast } from "~/features/todo/actions/addActionItemToToday"
import type { TaskActionItemSaveOperation } from "~/features/task/converters/taskFormValues2updateRequest"

const actionItemCompletionMutationKey = ["tasks", "action-item-completion"] as const

type Props = { children: ReactNode }

export function TaskActionProvider({ children }: Props) {
  const data = useContext(TaskAPIDataContext)!
  const queryClient = useQueryClient()
  const i18n = useI18n()
  const [createdTask, setCreatedTask] = useState<{ id: string; revision?: number; manualEstimatedMinutes?: number | null }>()
  const [needsRevisionRefresh, setNeedsRevisionRefresh] = useState(false)
  const baselineRevision = useRef<number | undefined>(undefined)
  const baselineManualEstimate = useRef<number | null | undefined>(undefined)
  const pendingActionItemConfirmation = useRef<{ taskId: string; operations: TaskActionItemSaveOperation[] } | null>(null)
  const pendingTaskUpdateConfirmation = useRef<{ taskId: string; request: ReturnType<typeof taskFormValues2taskUpdateRequest> } | null>(null)

  useEffect(() => {
    if (data.taskId && data.state === "ready" && baselineRevision.current === undefined && data.task) {
      baselineRevision.current = data.task.revision
      baselineManualEstimate.current = data.task.manual_estimated_minutes
    }
  }, [data.state, data.task, data.taskId])

  const completionMutation = useMutation({
    mutationKey: actionItemCompletionMutationKey,
    mutationFn: ({ taskId, actionItemId, completed, actualMinutes }: UpdateActionItemCompletionInput) => {
      return completed
        ? postTasksTaskIdActionItemsIdcomplete(taskId, actionItemId, actualMinutes === undefined ? undefined : { actual_minutes: actualMinutes })
        : postTasksTaskIdActionItemsIdreopen(taskId, actionItemId)
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

  const taskCompletionMutation = useMutation({
    mutationKey: actionItemCompletionMutationKey,
    mutationFn: ({ taskId, revision, completed, actualMinutes }: UpdateTaskCompletionInput) => {
      return completed
        ? postTasksIdcomplete(taskId, actualMinutes === undefined ? undefined : { actual_minutes: actualMinutes }, { headers: { "If-Match": `"${revision}"` } })
        : postTasksIdreopen(taskId, { headers: { "If-Match": `"${revision}"` } })
    },
    onSuccess: async (_response, input) => {
      notify.success(i18n("tasks.toast.updated"))
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
      queryClient.invalidateQueries({ queryKey: getGetTodayTodoQueryKey() }),
      queryClient.invalidateQueries({ queryKey: getListInboxQueryKey() }),
      queryClient.invalidateQueries({ queryKey: getGetProjectsQueryKey() }),
      queryClient.invalidateQueries({ queryKey: getGetTasksTaskIdActionItemsQueryKey(taskId) }),
    ])
  }

  async function submit(values: TaskFormValues, dirtyFields: TaskFormDirtyFields, confirmTodoRegeneration = false): Promise<TaskFormSaveResult> {
    return data.taskId
      ? submitEdit(values, dirtyFields, confirmTodoRegeneration)
      : submitCreate(values, dirtyFields, confirmTodoRegeneration)
  }

  async function submitCreate(values: TaskFormValues, dirtyFields: TaskFormDirtyFields, confirmTodoRegeneration: boolean): Promise<TaskFormSaveResult> {
    let task = createdTask
    const result: TaskFormSaveResult = { complete: true, actionItems: [] }
    let savedAnything = false
    let actionItemsSaved = false

    if (task && needsRevisionRefresh) {
      try {
        const latest = await getTasksId(task.id)
        if (!latest.task) throw new Error("Task response missing")
        if (task.manualEstimatedMinutes !== latest.task.manual_estimated_minutes) {
          result.complete = false
          result.conflict = true
          return result
        }
        task = { ...task, revision: latest.task.revision, manualEstimatedMinutes: latest.task.manual_estimated_minutes }
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
        task = { id: created.id ?? "", revision: created.revision, manualEstimatedMinutes: created.manual_estimated_minutes }
        if (!task.id) throw new Error("Task create response missing id")
        setCreatedTask(task)
        result.taskSaved = true
        result.taskResetValues = taskFormTaskValues(values, created)
        savedAnything = true
      } else {
        const draftRequest = taskFormValues2taskUpdateRequest(values, dirtyFields)
        const pendingRequest = pendingTaskUpdateConfirmation.current
        const usePendingRequest = confirmTodoRegeneration && pendingRequest?.taskId === task.id && sameTaskUpdateRequest(pendingRequest.request, draftRequest)
        if (pendingRequest?.taskId === task.id && !sameTaskUpdateRequest(pendingRequest.request, draftRequest)) pendingTaskUpdateConfirmation.current = null
        const request = usePendingRequest
          ? { ...pendingRequest!.request, confirm_todo_regeneration: true }
          : draftRequest
        if (Object.keys(request).length) {
          if (task.revision === undefined) {
            const latest = await getTasksId(task.id)
            if (!latest.task) throw new Error("Task response missing")
            task = { ...task, revision: latest.task.revision, manualEstimatedMinutes: latest.task.manual_estimated_minutes }
            setCreatedTask(task)
            setNeedsRevisionRefresh(false)
          }
          let updated
          try {
            updated = await patchTasksId(task.id, request, { headers: { "If-Match": `"${task.revision}"` } })
          } catch (cause) {
            if (isTodoRegenerationRequired(cause)) {
              pendingTaskUpdateConfirmation.current = { taskId: task.id, request: draftRequest }
              return { complete: false, regenerationConfirmationRequired: true, actionItems: [] }
            }
            throwSubmissionFailure(cause, i18n, true)
          }
          pendingTaskUpdateConfirmation.current = null
          task = { ...task, revision: updated.revision, manualEstimatedMinutes: updated.manual_estimated_minutes }
          setCreatedTask(task)
          result.taskSaved = true
          result.taskResetValues = taskFormTaskValues(values, updated)
          savedAnything = true
        }
      }
    } catch (cause) {
      throwSubmissionFailure(cause, i18n)
    }

    const operations = confirmTodoRegeneration && pendingActionItemConfirmation.current?.taskId === task.id
      ? pendingActionItemConfirmation.current.operations
      : taskFormValues2actionItemSaveOperations(values, dirtyFields)
    const itemResult = await saveTaskActionItems(task.id, operations, confirmTodoRegeneration, task.revision, task.manualEstimatedMinutes)
    result.actionItems = itemResult.outcomes
    result.regenerationConfirmationRequired = itemResult.regenerationConfirmationRequired
    result.conflict = itemResult.revisionConflict
    pendingActionItemConfirmation.current = itemResult.pendingOperations?.length
      ? { taskId: task.id, operations: itemResult.pendingOperations }
      : null
    savedAnything ||= itemResult.hasSavedItems
    actionItemsSaved = itemResult.hasSavedItems
    if (itemResult.outcomes.some((outcome) => outcome.error) || itemResult.regenerationConfirmationRequired || itemResult.revisionConflict) result.complete = false

    if (actionItemsSaved && !itemResult.revisionConflict) {
      try {
        const latest = await getTasksId(task.id)
        if (!latest.task) throw new Error("Task response missing")
        if (hasPendingManualEstimateDelete(itemResult.pendingOperations) && task.manualEstimatedMinutes !== latest.task.manual_estimated_minutes) {
          result.complete = false
          result.conflict = true
        } else {
          task = { ...task, revision: latest.task.revision, manualEstimatedMinutes: latest.task.manual_estimated_minutes }
          setCreatedTask(task)
          setNeedsRevisionRefresh(false)
        }
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

  async function submitEdit(values: TaskFormValues, dirtyFields: TaskFormDirtyFields, confirmTodoRegeneration: boolean): Promise<TaskFormSaveResult> {
    const task = data.task
    let revision = baselineRevision.current
    if (!task || revision === undefined) return { complete: false }
    const result: TaskFormSaveResult = { complete: true, actionItems: [] }
    let savedAnything = false
    const draftTaskRequest = taskFormValues2taskUpdateRequest(values, dirtyFields)
    const pendingRequest = pendingTaskUpdateConfirmation.current
    const usePendingRequest = confirmTodoRegeneration && pendingRequest?.taskId === task.id && sameTaskUpdateRequest(pendingRequest.request, draftTaskRequest)
    if (pendingRequest?.taskId === task.id && !sameTaskUpdateRequest(pendingRequest.request, draftTaskRequest)) pendingTaskUpdateConfirmation.current = null
    const taskRequest = usePendingRequest
      ? { ...pendingRequest!.request, confirm_todo_regeneration: true }
      : draftTaskRequest

    if (needsRevisionRefresh) {
      try {
        const latest = await getTasksId(task.id)
        if (!latest.task) throw new Error("Task response missing")
        if (baselineManualEstimate.current !== latest.task.manual_estimated_minutes) {
          result.complete = false
          result.conflict = true
          return result
        }
        revision = latest.task.revision
        queryClient.setQueryData(getGetTasksIdQueryKey(task.id), latest)
        baselineRevision.current = revision
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
        pendingTaskUpdateConfirmation.current = null
        queryClient.setQueryData(getGetTasksIdQueryKey(task.id), { ...data.taskQuery.data, task: updated })
        revision = updated.revision
        baselineRevision.current = updated.revision
        baselineManualEstimate.current = updated.manual_estimated_minutes
        result.taskSaved = true
        result.taskResetValues = taskFormTaskValues(values, updated)
        savedAnything = true
      } catch (cause) {
        if (isTodoRegenerationRequired(cause)) {
          pendingTaskUpdateConfirmation.current = { taskId: task.id, request: draftTaskRequest }
          return { complete: false, regenerationConfirmationRequired: true, actionItems: [] }
        }
        throwSubmissionFailure(cause, i18n, true)
      }
    }

    const operations = confirmTodoRegeneration && pendingActionItemConfirmation.current?.taskId === task.id
      ? pendingActionItemConfirmation.current.operations
      : taskFormValues2actionItemSaveOperations(values, dirtyFields)
    const itemResult = await saveTaskActionItems(task.id, operations, confirmTodoRegeneration, revision, baselineManualEstimate.current)
    result.actionItems = itemResult.outcomes
    result.regenerationConfirmationRequired = itemResult.regenerationConfirmationRequired
    result.conflict = itemResult.revisionConflict
    pendingActionItemConfirmation.current = itemResult.pendingOperations?.length
      ? { taskId: task.id, operations: itemResult.pendingOperations }
      : null
    savedAnything ||= itemResult.hasSavedItems
    if (itemResult.outcomes.some((outcome) => outcome.error) || itemResult.regenerationConfirmationRequired || itemResult.revisionConflict) result.complete = false

    if (itemResult.hasSavedItems && !itemResult.revisionConflict) {
      try {
        const latest = await getTasksId(task.id)
        if (!latest.task) throw new Error("Task response missing")
        if (hasPendingManualEstimateDelete(itemResult.pendingOperations) && baselineManualEstimate.current !== latest.task.manual_estimated_minutes) {
          result.complete = false
          result.conflict = true
        } else {
          queryClient.setQueryData(getGetTasksIdQueryKey(task.id), latest)
          baselineRevision.current = latest.task.revision
          baselineManualEstimate.current = latest.task.manual_estimated_minutes
          setNeedsRevisionRefresh(false)
        }
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
      queryClient.invalidateQueries({ queryKey: getGetTodayTodoQueryKey() }),
      queryClient.invalidateQueries({ queryKey: getListInboxQueryKey() }),
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
      baselineManualEstimate.current = latest.manual_estimated_minutes
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
    updateTaskCompletion: (input) => taskCompletionMutation.mutateAsync(input).then(() => undefined),
    addTaskToToday: async (taskId, priority) => {
      try {
        const snapshot = await addTaskToday(taskId)
        queryClient.setQueryData(getGetTodayTodoQueryKey(), snapshot)
        await Promise.all([invalidateTaskQueries(undefined, taskId), queryClient.invalidateQueries({ queryKey: getGetTodayTodoQueryKey() })])
        showAddedToTodayToast(i18n, priority, () => {
          void makeTaskUrgent(i18n, taskId).then(() => invalidateTaskQueries(undefined, taskId))
        })
      } catch (error) {
        notify.error(i18n(isAllDayScheduleBlocksActionItems(error) ? "today.toast.allDayBlocks" : "today.error"))
      }
    },
    addActionItemToToday: async (taskId, actionItemId, priority) => {
      try {
        const snapshot = await addActionItemToToday(actionItemId)
        queryClient.setQueryData(getGetTodayTodoQueryKey(), snapshot)
        await Promise.all([invalidateTaskQueries(undefined, taskId), queryClient.invalidateQueries({ queryKey: getGetTodayTodoQueryKey() })])
        showAddedToTodayToast(i18n, priority, () => {
          void makeActionItemUrgent(i18n, taskId, actionItemId).then(() => invalidateTaskQueries(undefined, taskId))
        })
      } catch (error) {
        notify.error(i18n(isAllDayScheduleBlocksActionItems(error) ? "today.toast.allDayBlocks" : "today.error"))
      }
    },
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
  actual_minutes?: number | null
}): NonNullable<TaskFormSaveResult["taskResetValues"]> {
  return {
    title: response.title ?? values.title,
    description: response.description ?? values.description,
    dueDate: response.due_date ?? values.dueDate,
    priority: typeof response.priority === "string" ? response.priority : response.priority?.value ?? values.priority,
    projectId: response.project_id ?? values.projectId,
    manualEstimate: response.manual_estimated_minutes === null ? "" : response.manual_estimated_minutes === undefined ? values.manualEstimate : minutesToDurationText(response.manual_estimated_minutes),
    actualMinutes: response.actual_minutes === null ? "" : response.actual_minutes === undefined ? values.actualMinutes : String(response.actual_minutes),
  }
}

function throwSubmissionFailure(cause: unknown, i18n: ReturnType<typeof useI18n>, detectConflict = false): never {
  if (detectConflict && isTaskRevisionConflict(cause)) throw createTaskRevisionConflict()
  const rawErrors = taskFieldErrors(cause)
  if (rawErrors) {
    const error = new Error("Task validation failed") as TaskFormSubmissionError
    error.fieldErrors = Object.fromEntries(Object.keys(rawErrors).map((field) => [field, "tasks.form.invalidField" as const]))
    throw error
  }
  notify.error(i18n(taskErrorMessageKey(cause)))
  throw cause
}

function isTaskRevisionConflict(cause: unknown): boolean {
  if (!(cause instanceof ApiError)) return false
  if (cause.status === 412) return true
  if (cause.status !== 409 || typeof cause.data !== "object" || cause.data === null) return false
  const details = (cause.data as { error?: { details?: unknown } }).error?.details
  return Array.isArray(details) && details.some((detail) => typeof detail === "object" && detail !== null && ["revision_mismatch", "revision_conflict", "task_revision_conflict", "precondition_failed"].includes(String((detail as { code?: unknown }).code)))
}

function createTaskRevisionConflict(): TaskFormSubmissionError {
  return Object.assign(new Error("Task revision changed"), { conflict: true as const })
}

function hasPendingManualEstimateDelete(operations: TaskActionItemSaveOperation[] | undefined): boolean {
  return operations?.some((operation) => operation.type === "delete" && operation.manualEstimatedMinutes !== undefined) ?? false
}

function sameTaskUpdateRequest(left: ReturnType<typeof taskFormValues2taskUpdateRequest>, right: ReturnType<typeof taskFormValues2taskUpdateRequest>): boolean {
  const leftEntries = Object.entries(left).sort(([a], [b]) => a.localeCompare(b))
  const rightEntries = Object.entries(right).sort(([a], [b]) => a.localeCompare(b))
  return JSON.stringify(leftEntries) === JSON.stringify(rightEntries)
}
