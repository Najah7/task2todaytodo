import { getTodayTodo, addTodayTodoActionItem, addTodayTodoTask, type RestTodoTodayResponse } from "~/api/generated/todo"
import { getTasksId, patchTasksId, patchTasksTaskIdActionItemsId } from "~/api/generated/tasks"
import { ApiError } from "~/api/http"
import { notify } from "~/features/shared/notification"
import type { MessageKey } from "~/features/i18n/messages/types"
import { todoRevisionHeaders } from "./revisionHeaders"

export async function addActionItemToToday(actionItemId: string): Promise<RestTodoTodayResponse> {
  const todo = await getTodayTodo()
  if (todo.revision === undefined) throw new Error("Today revision is missing")
  return addTodayTodoActionItem(actionItemId, { headers: todoRevisionHeaders(todo.revision) })
}

export async function addTaskToToday(taskId: string): Promise<RestTodoTodayResponse> {
  const todo = await getTodayTodo()
  if (todo.revision === undefined) throw new Error("Today revision is missing")
  return addTodayTodoTask(taskId, { headers: todoRevisionHeaders(todo.revision) })
}

export function isAllDayScheduleBlocksActionItems(error: unknown): boolean {
  if (!(error instanceof ApiError) || typeof error.data !== "object" || error.data === null) return false
  const details = (error.data as { error?: { details?: unknown } }).error?.details
  return Array.isArray(details) && details.some((detail) => typeof detail === "object" && detail !== null && (detail as { code?: unknown }).code === "all_day_schedule_blocks_action_items")
}

export function showAddedToTodayToast(
  t: (key: MessageKey, params?: Record<string, string | number>) => string,
  priority: string | undefined,
  onMakeUrgent: () => void,
) {
  if (priority === "urgent") {
    notify.success(t("today.toast.added"))
    return
  }
  notify.action(t("today.toast.added"), t("today.toast.makeUrgent"), () => {
    onMakeUrgent()
  })
}

export async function makeActionItemUrgent(t: (key: MessageKey, params?: Record<string, string | number>) => string, taskId: string, actionItemId: string) {
  try {
    await patchTasksTaskIdActionItemsId(taskId, actionItemId, { priority: "urgent" })
  } catch {
    notify.error(t("today.toast.urgentError"))
    return
  }
  notify.success(t("today.toast.urgentUpdated"))
}

export async function makeTaskUrgent(t: (key: MessageKey, params?: Record<string, string | number>) => string, taskId: string) {
  try {
    const { task } = await getTasksId(taskId)
    if (!task) throw new Error("Task response missing")
    await patchTasksId(taskId, { priority: "urgent" }, { headers: { "If-Match": `"${task.revision}"` } })
  } catch {
    notify.error(t("today.toast.urgentError"))
    return
  }
  notify.success(t("today.toast.urgentUpdated"))
}
