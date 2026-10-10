import type { FieldNamesMarkedBoolean } from "react-hook-form"
import type { RestActionItemCreateRequest, RestActionItemUpdateRequest, RestTaskUpdateRequest } from "~/api/generated/tasks"
import { durationTextToMinutes, type TaskFormValues } from "~/features/task/components/TaskForm/schema"

export type ActionItemFormValue = TaskFormValues["actionItems"][number]
export type TaskFormDirtyFields = FieldNamesMarkedBoolean<TaskFormValues>

export type TaskActionItemSaveOperation =
  | { type: "create"; key: string; index: number; item: ActionItemFormValue; data: RestActionItemCreateRequest }
  | { type: "update"; key: string; index: number; item: ActionItemFormValue; seriesId: string; data: RestActionItemUpdateRequest }
  | { type: "delete"; key: string; index: number; item: ActionItemFormValue }
  | { type: "discard"; key: string; index: number }

export function taskFormValues2taskUpdateRequest(values: TaskFormValues, dirtyFields: TaskFormDirtyFields): RestTaskUpdateRequest {
  const request: RestTaskUpdateRequest = {}
  if (dirtyFields.title) request.title = values.title.trim()
  if (dirtyFields.description) request.description = values.description
  if (dirtyFields.dueDate) request.due_date = values.dueDate || null
  if (dirtyFields.priority) request.priority = values.priority
  if (dirtyFields.projectId) request.project_id = values.projectId || null
  if (dirtyFields.manualEstimate) request.manual_estimated_minutes = durationTextToMinutes(values.manualEstimate) ?? null
  return request
}

export function taskFormValues2actionItemSaveOperations(values: TaskFormValues, dirtyFields: TaskFormDirtyFields): TaskActionItemSaveOperation[] {
  const dirtyItems = Array.isArray(dirtyFields.actionItems) ? dirtyFields.actionItems : []
  const operations: TaskActionItemSaveOperation[] = []
  for (const [index, item] of values.actionItems.entries()) {
    const key = actionItemKey(item, index)
    if (item.deletionSucceeded) continue
    if (item.pendingDelete) {
      operations.push(item.seriesId ? { type: "delete", key, index, item } : { type: "discard", key, index })
      continue
    }
    if (!item.seriesId) {
      operations.push({ type: "create", key, index, item, data: actionItemValues2createRequest(item) })
      continue
    }

    const dirtyItem = dirtyItems[index]
    if (!dirtyItem) continue
    const data: RestActionItemUpdateRequest = { scope: "current", occurrence_date: item.occurrenceDate }
    let changed = false
    if (dirtyItem.title) { data.title = item.title.trim(); changed = true }
    if (dirtyItem.estimatedMinutes) { data.estimated_minutes = durationTextToMinutes(item.estimatedMinutes) ?? null; changed = true }
    if (dirtyItem.priority) { data.priority = item.priority; changed = true }
    if (changed) operations.push({ type: "update", key, index, item, seriesId: item.seriesId, data })
  }
  return operations
}

export function actionItemKey(item: ActionItemFormValue, index: number): string {
  return item.clientKey ?? (item.seriesId && item.occurrenceDate
    ? `persisted:${item.seriesId}:${item.occurrenceDate}`
    : `new:${index}`)
}

function actionItemValues2createRequest(item: ActionItemFormValue): RestActionItemCreateRequest {
  const estimatedMinutes = durationTextToMinutes(item.estimatedMinutes)
  return {
    title: item.title.trim(),
    ...(estimatedMinutes === undefined ? {} : { estimated_minutes: estimatedMinutes }),
    ...(item.priority ? { priority: item.priority } : {}),
  }
}
