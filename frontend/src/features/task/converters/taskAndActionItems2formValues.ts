import type { RestActionItemResponse, RestTaskResponse } from "~/api/generated/tasks"
import type { TaskFormValues } from "~/features/task/components/TaskForm/schema"
import { minutesToDurationText } from "~/features/task/components/TaskForm/schema"

export function taskAndActionItems2formValues(inputs: { task: RestTaskResponse; actionItems: RestActionItemResponse[] }[]): TaskFormValues[] {
  return inputs.map(({ task, actionItems }) => ({
    title: task.title,
    projectId: task.project_id ?? "",
    dueDate: task.due_date ?? "",
    description: task.description,
    priority: task.priority,
    manualEstimate: minutesToDurationText(task.manual_estimated_minutes),
    actionItems: actionItems.map((item) => ({
      seriesId: item.series_id ?? item.id ?? "",
      occurrenceDate: item.occurrence_date ?? "",
      clientKey: `persisted:${item.series_id ?? item.id ?? ""}:${item.occurrence_date ?? ""}`,
      isRecurring: Boolean(item.series_id && ((item.interval_weeks ?? 0) > 0 || item.repeat_state === "active" || item.repeat_state === "paused")),
      completed: item.completed ?? false,
      title: item.title ?? "",
      estimatedMinutes: minutesToDurationText(item.estimated_minutes),
      priority: item.priority ?? "",
    })),
  }))
}
