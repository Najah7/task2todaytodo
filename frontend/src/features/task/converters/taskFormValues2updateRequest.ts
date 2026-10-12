import type { FieldNamesMarkedBoolean } from "react-hook-form"
import type { RestActionItemCreateRequest, RestActionItemUpdateRequest, RestTaskUpdateRequest } from "~/api/generated/tasks"
import { durationTextToMinutes, type TaskFormValues } from "~/features/task/components/TaskForm/schema"

export type ActionItemFormValue = TaskFormValues["actionItems"][number]
export type TaskFormDirtyFields = FieldNamesMarkedBoolean<TaskFormValues>

export type TaskActionItemSaveOperation =
  | { type: "create"; key: string; index: number; item: ActionItemFormValue; data: RestActionItemCreateRequest }
  | { type: "update"; key: string; index: number; item: ActionItemFormValue; actionItemId: string; data: RestActionItemUpdateRequest }
  | { type: "delete"; key: string; index: number; item: ActionItemFormValue; manualEstimatedMinutes?: number }
  | { type: "discard"; key: string; index: number }

export function taskFormValues2taskUpdateRequest(values: TaskFormValues, dirtyFields: TaskFormDirtyFields): RestTaskUpdateRequest {
  const request: RestTaskUpdateRequest = {}
  if (dirtyFields.title) request.title = values.title.trim()
  if (dirtyFields.description) request.description = values.description
  if (dirtyFields.dueDate) request.due_date = values.dueDate || null
  if (dirtyFields.priority) request.priority = values.priority
  if (dirtyFields.projectId) request.project_id = values.projectId || null
  const activeItems = values.actionItems.filter((item) => !item.pendingDelete && !item.deletionSucceeded)
  const deletingLastActionItem = activeItems.length === 0 && values.actionItems.some((item) => item.pendingDelete && item.id)
  if (dirtyFields.manualEstimate && activeItems.length === 0 && !deletingLastActionItem) {
    request.manual_estimated_minutes = durationTextToMinutes(values.manualEstimate) ?? null
  }
  if (dirtyFields.actualMinutes && values.taskCompleted && activeItems.length === 0) request.actual_minutes = actualMinutesText2number(values.actualMinutes ?? "")
  return request
}

export function taskFormValues2actionItemSaveOperations(values: TaskFormValues, dirtyFields: TaskFormDirtyFields): TaskActionItemSaveOperation[] {
  const dirtyItems = Array.isArray(dirtyFields.actionItems) ? dirtyFields.actionItems : []
  const operations: TaskActionItemSaveOperation[] = []
  const activeItems = values.actionItems.filter((item) => !item.pendingDelete && !item.deletionSucceeded)
  const manualEstimateForLastDelete = activeItems.length === 0 ? durationTextToMinutes(values.manualEstimate) : undefined
  let lastDeleteIndex = -1
  if (manualEstimateForLastDelete !== undefined) {
    for (const [index, item] of values.actionItems.entries()) {
      if (item.pendingDelete && item.id) lastDeleteIndex = index
    }
  }
  for (const [index, item] of values.actionItems.entries()) {
    const key = actionItemKey(item, index)
    if (item.deletionSucceeded) continue
    if (item.pendingDelete) {
      operations.push(item.id ? {
        type: "delete", key, index, item,
        ...(index === lastDeleteIndex && manualEstimateForLastDelete !== undefined ? { manualEstimatedMinutes: manualEstimateForLastDelete } : {}),
      } : { type: "discard", key, index })
      continue
    }
    if (!item.id) {
      operations.push({ type: "create", key, index, item, data: actionItemValues2createRequest(item) })
      continue
    }

    const dirtyItem = dirtyItems[index]
    if (!dirtyItem) continue
    const data: RestActionItemUpdateRequest = {}
    let changed = false
    if (dirtyItem.title) { data.title = item.title.trim(); changed = true }
    if (dirtyItem.estimatedMinutes) { data.estimated_minutes = durationTextToMinutes(item.estimatedMinutes) ?? 0; changed = true }
    if (dirtyItem.actualMinutes) { data.actual_minutes = actualMinutesText2number(item.actualMinutes ?? ""); changed = true }
    if (dirtyItem.priority) { data.priority = item.priority; changed = true }
    if (changed) operations.push({ type: "update", key, index, item, actionItemId: item.id, data })
  }
  return operations.sort((left, right) => operationOrder(left.type) - operationOrder(right.type))
}

function operationOrder(type: TaskActionItemSaveOperation["type"]): number {
  if (type === "create") return 0
  if (type === "update") return 1
  if (type === "delete") return 2
  return 3
}

function actualMinutesText2number(value: string): number | null {
  const text = value.trim()
  if (!text) return null
  const minutes = Number(text)
  return Number.isSafeInteger(minutes) && minutes >= 0 ? minutes : null
}

export function actionItemKey(item: ActionItemFormValue, index: number): string {
  return item.clientKey ?? (item.id ? `persisted:${item.id}` : `new:${index}`)
}

function actionItemValues2createRequest(item: ActionItemFormValue): RestActionItemCreateRequest {
  const estimatedMinutes = durationTextToMinutes(item.estimatedMinutes)
  return {
    title: item.title.trim(),
    estimated_minutes: estimatedMinutes ?? 0,
    ...(item.priority ? { priority: item.priority } : {}),
  }
}
