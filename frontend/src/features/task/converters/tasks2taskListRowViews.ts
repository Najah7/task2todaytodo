import type { RestActionItemResponse, RestTaskResponse } from "~/api/generated/tasks"
import type { TaskListRowView } from "~/features/task/types"

type Input = {
  task: RestTaskResponse
  actionItems: RestActionItemResponse[]
  language: string
  unassignedProject: string
  noDeadline: string
  getStatusLabel: (status: string) => string
}

export function tasks2taskListRowViews(inputs: Input[]): TaskListRowView[] {
  return inputs.map(({ task, actionItems, language, unassignedProject, noDeadline, getStatusLabel }) => {
    const id = task.id ?? ""
    const status = task.status ?? "open"
    const statusTone = status === "in_progress" ? "active" : status === "waiting_on_others" ? "waiting" : status === "done" ? "done" : "open"
    const remainingDays = task.remaining_days
    const dueDateTone = !task.due_date
      ? "none"
      : remainingDays === null || remainingDays === undefined
        ? "normal"
        : remainingDays < 0
          ? "overdue"
          : remainingDays === 0
            ? "today"
            : "normal"
    return {
      id,
      title: task.title ?? "",
      canUpdate: task.can_update,
      projectLabel: task.project_name || unassignedProject,
      statusLabel: getStatusLabel(status),
      statusTone,
      actionItemProgressLabel: `${task.action_item_completed_count ?? 0} / ${task.action_item_count ?? 0}`,
      actionItemCount: task.action_item_count ?? 0,
      actionItemCompletedCount: task.action_item_completed_count ?? 0,
      progress: task.progress ?? 0,
      estimateLabel: durationLabel(task.estimated_minutes),
      dueDateLabel: task.due_date ? formatDate(task.due_date, language) : noDeadline,
      dueDateTone,
      actionItems: actionItems.map((item) => {
        const taskId = item.task_id || id
        const seriesId = item.series_id || undefined
        const occurrenceDate = item.occurrence_date || undefined
        const isRecurring = Boolean(seriesId && ((item.interval_weeks ?? 0) > 0 || item.repeat_state === "active" || item.repeat_state === "paused"))
        const actionItemId = seriesId ?? item.id ?? ""
        return {
          occurrenceKey: `${seriesId ?? actionItemId}:${occurrenceDate ?? ""}`,
          taskId,
          actionItemId,
          ...(seriesId ? { seriesId } : {}),
          ...(occurrenceDate ? { occurrenceDate } : {}),
          title: item.title ?? "",
          estimateLabel: durationLabel(item.estimated_minutes),
          completed: item.completed ?? false,
          occurrenceDateLabel: occurrenceDate && isRecurring ? formatDate(occurrenceDate, language) : "",
          todayRegistration: null,
        }
      }),
    }
  })
}

function durationLabel(minutes: number | null | undefined): string {
  if (minutes === null || minutes === undefined || !Number.isSafeInteger(minutes) || minutes < 0) return "—"
  return `${Math.floor(minutes / 60)}:${String(minutes % 60).padStart(2, "0")}`
}

function formatDate(date: string, language: string): string {
  const value = new Date(`${date}T00:00:00.000Z`)
  return new Intl.DateTimeFormat(language === "ja" ? "ja-JP" : "en-US", {
    year: language === "ja" ? undefined : "numeric",
    month: "numeric",
    day: "numeric",
    weekday: language === "ja" ? "short" : undefined,
    timeZone: "UTC",
  }).format(value)
}
