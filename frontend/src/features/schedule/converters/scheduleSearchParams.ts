import type { ScheduleNavigationState, ScheduleView } from "~/features/schedule/providers/ScheduleNavigationProvider/context"

const views: ScheduleView[] = ["day", "week", "month"]

export function scheduleSearchParams2state(searchParams: URLSearchParams, today: string): ScheduleNavigationState {
  const rawView = searchParams.get("view")
  const view = views.includes(rawView as ScheduleView) ? rawView as ScheduleView : "week"
  const date = searchParams.get("date")
  const projectId = searchParams.get("project_id") || undefined
  return {
    date: isCalendarDate(date) ? date : today,
    view,
    projectId,
    showSkipped: searchParams.get("show_skipped") === "true",
  }
}

export function scheduleState2searchParams(state: ScheduleNavigationState, previous: URLSearchParams): URLSearchParams {
  const next = new URLSearchParams(previous)
  next.set("date", state.date)
  next.set("view", state.view)
  if (state.projectId) next.set("project_id", state.projectId)
  else next.delete("project_id")
  if (state.showSkipped) next.set("show_skipped", "true")
  else next.delete("show_skipped")
  next.delete("page_token")
  return next
}

function isCalendarDate(value: string | null): value is string {
  if (!value || !/^\d{4}-\d{2}-\d{2}$/.test(value)) return false
  const [year, month, day] = value.split("-").map(Number)
  const date = new Date(Date.UTC(year!, month! - 1, day!))
  return date.getUTCFullYear() === year && date.getUTCMonth() === month! - 1 && date.getUTCDate() === day
}
