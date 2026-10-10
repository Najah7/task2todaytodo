import {
  deleteTasksTaskIdActionItemsId,
  patchTasksTaskIdActionItemsId,
  postTasksTaskIdActionItems,
  postTasksTaskIdActionItemsIdskip,
  type RestActionItemResponse,
} from "~/api/generated/tasks"
import type { TaskFormSaveResult } from "~/features/task/components/TaskForm"
import { minutesToDurationText, type TaskFormValues } from "~/features/task/components/TaskForm/schema"
import { actionItemFieldErrors } from "~/features/task/errors"
import type { TaskActionItemSaveOperation } from "~/features/task/converters/taskFormValues2updateRequest"

export type ActionItemSaveOutcome = NonNullable<TaskFormSaveResult["actionItems"]>[number]

export async function saveTaskActionItems(taskId: string, operations: TaskActionItemSaveOperation[]): Promise<{ outcomes: ActionItemSaveOutcome[]; hasSavedItems: boolean }> {
  const outcomes: ActionItemSaveOutcome[] = []
  let hasSavedItems = false
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
          identity: { seriesId: created.series_id ?? created.id ?? "", occurrenceDate: created.occurrence_date ?? created.due_date ?? "" },
          savedFields: responseSavedFields(created, operation.item),
        })
      } else if (operation.type === "update") {
        const updated = await patchTasksTaskIdActionItemsId(taskId, operation.seriesId, operation.data)
        outcomes.push({ index: operation.index, key: operation.key, savedFields: responseSavedFields(updated, operation.item) })
      } else {
        if (operation.item.isRecurring) {
          await postTasksTaskIdActionItemsIdskip(taskId, operation.item.seriesId!, { occurrence_date: operation.item.occurrenceDate })
        } else {
          await deleteTasksTaskIdActionItemsId(taskId, operation.item.seriesId!)
        }
        outcomes.push({ index: operation.index, key: operation.key, deletionSucceeded: true })
      }
      hasSavedItems = true
    } catch (error) {
      outcomes.push({
        index: operation.index,
        key: operation.key,
        error: "tasks.form.actionItemSaveError",
        fieldErrors: actionItemFieldErrors(error),
      })
    }
  }
  return { outcomes, hasSavedItems }
}

function responseSavedFields(response: RestActionItemResponse, fallback: TaskFormValues["actionItems"][number]): ActionItemSaveOutcome["savedFields"] {
  return {
    title: response.title ?? fallback.title,
    estimatedMinutes: response.estimated_minutes === null
      ? ""
      : response.estimated_minutes === undefined
        ? fallback.estimatedMinutes
        : minutesToDurationText(response.estimated_minutes),
    priority: response.priority ?? fallback.priority,
  }
}
