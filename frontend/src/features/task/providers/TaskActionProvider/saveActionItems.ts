import {
  deleteTasksTaskIdActionItemsId,
  patchTasksTaskIdActionItemsId,
  postTasksTaskIdActionItems,
  type RestActionItemResponse,
} from "~/api/generated/tasks"
import { getTasksId } from "~/api/generated/tasks"
import type { TaskFormSaveResult } from "./context"
import { minutesToDurationText, type TaskFormValues } from "~/features/task/components/TaskForm/schema"
import { actionItemFieldErrors, taskErrorMessageKey } from "~/features/task/errors"
import type { TaskActionItemSaveOperation } from "~/features/task/converters/taskFormValues2updateRequest"
import { ApiError } from "~/api/http"

export type ActionItemSaveOutcome = NonNullable<TaskFormSaveResult["actionItems"]>[number]

export async function saveTaskActionItems(taskId: string, operations: TaskActionItemSaveOperation[], initialTaskRevision?: number, initialManualEstimate?: number | null): Promise<{ outcomes: ActionItemSaveOutcome[]; hasSavedItems: boolean; revisionConflict?: true; pendingOperations?: TaskActionItemSaveOperation[] }> {
  const outcomes: ActionItemSaveOutcome[] = []
  let hasSavedItems = false
  let taskRevision = initialTaskRevision
  let taskRevisionNeedsRefresh = false
  let replacementCreateFailed = false
  const hasReplacementCreate = operations.some((operation) => operation.type === "create") && operations.some((operation) => operation.type === "delete")
  for (const operation of operations) {
    if (operation.type === "discard") {
      outcomes.push({ index: operation.index, key: operation.key, deletionSucceeded: true })
      continue
    }
    try {
      if (operation.type === "create") {
        const created = await postTasksTaskIdActionItems(taskId, operation.data)
        outcomes.push({
          index: operation.index,
          key: operation.key,
          identity: { id: created.id ?? "" },
          savedFields: responseSavedFields(created, operation.item),
        })
      } else if (operation.type === "update") {
        const updated = await patchTasksTaskIdActionItemsId(taskId, operation.actionItemId, operation.data)
        outcomes.push({ index: operation.index, key: operation.key, savedFields: responseSavedFields(updated, operation.item) })
      } else {
        // Keep the existing children if a replacement could not be created.
        // Deleting them could transiently turn the Task into direct work and
        // commit a structural change the user did not intend.
        if (replacementCreateFailed && hasReplacementCreate) continue
        const request = {
          ...(operation.manualEstimatedMinutes === undefined ? {} : { manual_estimated_minutes: operation.manualEstimatedMinutes }),
        }
        const options = operation.manualEstimatedMinutes === undefined ? undefined : { headers: { "If-Match": `"${await currentTaskRevision()}"` } }
        await deleteTasksTaskIdActionItemsId(taskId, operation.item.id!, Object.keys(request).length ? request : undefined, options)
        outcomes.push({ index: operation.index, key: operation.key, deletionSucceeded: true })
      }
      hasSavedItems = true
      if (operation.type === "create" || operation.type === "update" || operation.type === "delete") taskRevisionNeedsRefresh = true
    } catch (error) {
      if (operation.type === "create" && hasReplacementCreate) replacementCreateFailed = true
      if (isTaskRevisionConflict(error)) return { outcomes, hasSavedItems, revisionConflict: true, pendingOperations: operations.slice(outcomes.length) }
      const messageKey = taskErrorMessageKey(error)
      outcomes.push({
        index: operation.index,
        key: operation.key,
        error: messageKey === "tasks.form.freeProjectLimit" || messageKey === "tasks.form.freeActionItemLimit"
          ? messageKey
          : "tasks.form.actionItemSaveError",
        fieldErrors: actionItemFieldErrors(error),
      })
    }
  }
  return { outcomes, hasSavedItems }

  async function currentTaskRevision(): Promise<number> {
    if (taskRevisionNeedsRefresh || taskRevision === undefined) {
      const latest = await getTasksId(taskId)
      if (latest.task?.revision === undefined) throw new Error("Task revision is missing")
      if (initialManualEstimate !== undefined && latest.task.manual_estimated_minutes !== initialManualEstimate) throw new TaskManualEstimateChangedError()
      taskRevision = latest.task.revision
      taskRevisionNeedsRefresh = false
    }
    return taskRevision
  }
}

class TaskManualEstimateChangedError extends Error {}

function isTaskRevisionConflict(error: unknown): boolean {
  if (error instanceof TaskManualEstimateChangedError) return true
  if (!(error instanceof ApiError)) return false
  if (error.status === 412) return true
  if (error.status !== 409 || typeof error.data !== "object" || error.data === null) return false
  const details = (error.data as { error?: { details?: unknown } }).error?.details
  return Array.isArray(details) && details.some((detail) => typeof detail === "object" && detail !== null && ["revision_conflict", "task_revision_conflict", "precondition_failed"].includes(String((detail as { code?: unknown }).code)))
}

function responseSavedFields(response: RestActionItemResponse, fallback: TaskFormValues["actionItems"][number]): ActionItemSaveOutcome["savedFields"] {
  return {
    title: response.title ?? fallback.title,
    estimatedMinutes: response.estimated_minutes === null
      ? ""
      : response.estimated_minutes === undefined
        ? fallback.estimatedMinutes
        : minutesToDurationText(response.estimated_minutes),
    actualMinutes: response.actual_minutes == null ? "" : String(response.actual_minutes),
    priority: response.priority ?? fallback.priority,
  }
}
