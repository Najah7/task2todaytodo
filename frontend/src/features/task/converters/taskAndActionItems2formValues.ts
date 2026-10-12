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
    taskCompleted: task.status === "done",
    taskStarted: task.started,
    taskEstimateEditable: task.estimate_editable,
    canAddActionItem: task.can_add_action_item,
    actualMinutes: task.actual_minutes == null ? "" : String(task.actual_minutes),
    actionItems: actionItems.map((item) => ({
      id: item.id ?? "",
      clientKey: `persisted:${item.id ?? ""}`,
      completed: item.completed ?? false,
      started: item.started ?? false,
      estimateEditable: item.estimate_editable ?? true,
      actualMinutes: item.actual_minutes == null ? "" : String(item.actual_minutes),
      title: item.title ?? "",
      estimatedMinutes: minutesToDurationText(item.estimated_minutes),
      priority: item.priority ?? "",
    })),
  }))
}
