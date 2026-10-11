import type { RestInboxSchedulePayloadResponse } from "~/api/generated/inbox"
import { addCalendarDays, instantToLocalDate } from "~/features/schedule/lib/dateTime"

function formatLocalDate(value: string, locale: string) {
  const date = new Date(`${value}T12:00:00Z`)
  return new Intl.DateTimeFormat(locale, { month: "numeric", day: "numeric", weekday: "short", timeZone: "UTC" }).format(date)
}

function formatTime(value: string, timezone: string, locale: string) {
  return new Intl.DateTimeFormat(locale, { hour: "numeric", minute: "2-digit", timeZone: timezone }).format(new Date(value))
}

export function formatInboxSchedule(schedule: RestInboxSchedulePayloadResponse, userTimezone: string, locale: string, allDayLabel: string) {
  const displayTimezone = schedule.all_day ? schedule.timezone : userTimezone
  const startDateValue = instantToLocalDate(schedule.start_at, displayTimezone)
  const endDateValue = instantToLocalDate(schedule.end_at, displayTimezone)
  const inclusiveEndDateValue = schedule.all_day ? addCalendarDays(endDateValue, -1) : endDateValue
  const startDate = formatLocalDate(startDateValue, locale)
  const endDate = formatLocalDate(inclusiveEndDateValue, locale)
  if (schedule.all_day) return startDate === endDate ? `${startDate} · ${allDayLabel}` : `${startDate} – ${endDate} · ${allDayLabel}`
  const start = formatTime(schedule.start_at, displayTimezone, locale)
  const end = formatTime(schedule.end_at, displayTimezone, locale)
  return startDate === endDate ? `${startDate} · ${start}–${end}` : `${startDate} ${start} – ${endDate} ${end}`
}
