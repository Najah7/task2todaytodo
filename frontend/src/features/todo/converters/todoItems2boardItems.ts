import type { RestTodoItemResponse } from "~/api/generated/todo"
import type { TodayBoardItem } from "~/features/todo/components/TodayBoard/types"

export function todoItems2boardItems(items: RestTodoItemResponse[], timezone: string, locale: string): TodayBoardItem[] {
  return items.map((item) => {
    const history = item.history === true
    const completed = item.completed === true
    const kind = item.kind === "schedule" ? "schedule" : item.kind === "task" ? "task" : "action_item"
    const status = history ? "today.status.history" : completed ? "today.status.completed" : item.started ? "today.status.inProgress" : kind === "schedule" ? "today.status.scheduled" : "today.status.notScheduled"
    const groupLabel = [item.project_title, kind === "action_item" ? item.task_title : undefined].filter(Boolean).join(" · ")
    return {
      id: item.id || "",
      kind,
      title: item.title || "",
      groupLabel,
      status,
      timeLabel: history ? "—" : timeLabel(item.scheduled_start, timezone, locale),
      durationLabel: minutesLabel(kind === "schedule"
        ? item.duration_minutes ?? intervalMinutes(item.scheduled_start, item.scheduled_end)
        : item.planned_minutes),
      completed,
      history,
      canComplete: item.can_complete === true,
      canReopen: item.can_reopen === true,
      canMove: item.can_move === true && !history && !completed,
      canRemove: item.can_remove === true,
      canHide: item.can_hide === true,
      removalBlockedMessage: item.started && !item.can_remove ? "today.item.startedRemoveWarning" : undefined,
    }
  })
}

export function civilDateLabel(value: string | undefined, locale: string) {
  if (!value) return ""
  const [year, month, day] = value.split("-").map(Number)
  const date = new Date(Date.UTC(year, month - 1, day, 12))
  return new Intl.DateTimeFormat(locale, { weekday: "long", year: "numeric", month: "long", day: "numeric", timeZone: "UTC" }).format(date)
}

export function timeLabel(value: string | null | undefined, timezone: string, locale: string) {
  if (!value) return "—"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return "—"
  return new Intl.DateTimeFormat(locale, { timeZone: timezone, hour: "2-digit", minute: "2-digit", hourCycle: "h23" }).format(date)
}

export function minutesLabel(minutes?: number | null) {
  if (minutes === undefined || minutes === null) return "—"
  return `${Math.floor(minutes / 60)}:${String(minutes % 60).padStart(2, "0")}`
}

function intervalMinutes(start?: string | null, end?: string | null): number | null {
  if (!start || !end) return null
  const minutes = Math.round((new Date(end).getTime() - new Date(start).getTime()) / 60_000)
  return Number.isFinite(minutes) && minutes >= 0 ? minutes : null
}
