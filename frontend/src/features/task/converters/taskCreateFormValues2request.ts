import type { RestTaskCreateRequest } from "~/api/generated/tasks"
import { durationTextToMinutes, type TaskCreateFormValues } from "~/features/task/components/TaskForm/schema"

export type TaskCreateRequest = RestTaskCreateRequest

export function taskCreateFormValues2requests(valuesList: TaskCreateFormValues[]): TaskCreateRequest[] {
  return valuesList.map((values) => {
    const activeItems = values.actionItems.filter((item) => !item.pendingDelete && !item.deletionSucceeded)
    const manualEstimate = activeItems.length > 0
      ? activeItems.reduce((total, item) => total + (durationTextToMinutes(item.estimatedMinutes) ?? 0), 0)
      : durationTextToMinutes(values.manualEstimate) ?? 0
    return {
      title: values.title.trim(),
      description: values.description,
      ...(values.dueDate ? { due_date: values.dueDate } : {}),
      ...(values.priority ? { priority: values.priority } : {}),
      manual_estimated_minutes: manualEstimate,
    }
  })
}
