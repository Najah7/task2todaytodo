import type { RestProjectTaskCreateRequest } from "~/api/generated/tasks"
import { durationTextToMinutes, type TaskCreateFormValues } from "~/features/task/components/TaskForm/schema"

export type TaskCreateRequest = RestProjectTaskCreateRequest

export function taskCreateFormValues2requests(valuesList: TaskCreateFormValues[]): TaskCreateRequest[] {
  return valuesList.map((values) => ({
    title: values.title.trim(),
    description: values.description,
    ...(values.dueDate ? { due_date: values.dueDate } : {}),
    ...(values.priority ? { priority: values.priority } : {}),
    ...(durationTextToMinutes(values.manualEstimate) === undefined ? {} : { manual_estimated_minutes: durationTextToMinutes(values.manualEstimate) }),
  }))
}
