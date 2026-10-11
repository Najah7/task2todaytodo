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
      id: item.id ?? "",
      clientKey: `persisted:${item.id ?? ""}`,
      completed: item.completed ?? false,
      title: item.title ?? "",
      estimatedMinutes: minutesToDurationText(item.estimated_minutes),
      priority: item.priority ?? "",
    })),
  }))
}
