import { scheduleRange } from "~/features/schedule/lib/scheduleRange"
import type { ScheduleEvent } from "~/features/schedule/providers/ScheduleAPIDataProvider/context"
import type { ScheduleView } from "~/features/schedule/providers/ScheduleNavigationProvider/context"

const minutesPerDay = 24 * 60

function parseDate(value: string) {
  const [year = 1970, month = 1, day = 1] = value.split("-").map(Number)
  return new Date(Date.UTC(year, month - 1, day))
}

export function dateKey(value: Date) {
  return `${value.getUTCFullYear()}-${String(value.getUTCMonth() + 1).padStart(2, "0")}-${String(value.getUTCDate()).padStart(2, "0")}`
}

export function calendarDates(anchor: string, view: ScheduleView, timezone: string) {
  const range = scheduleRange(anchor, view, timezone)
  const start = parseDate(range.startDate)
  const end = parseDate(range.endDateExclusive)
  return Array.from({ length: (end.getTime() - start.getTime()) / 86_400_000 }, (_, index) => {
    const date = new Date(start)
    date.setUTCDate(date.getUTCDate() + index)
    return date
  })
}

export function formatDate(value: Date, language: string, options: Intl.DateTimeFormatOptions) {
  return new Intl.DateTimeFormat(language === "ja" ? "ja-JP" : "en-US", { ...options, timeZone: "UTC" }).format(value)
}

export function zonedDateKey(value: Date, timezone: string) {
  const parts = new Intl.DateTimeFormat("en-CA", { timeZone: timezone, year: "numeric", month: "2-digit", day: "2-digit" }).formatToParts(value)
  const part = (type: Intl.DateTimeFormatPartTypes) => parts.find((item) => item.type === type)?.value ?? "00"
  return `${part("year")}-${part("month")}-${part("day")}`
}

export function zonedTime(value: Date, timezone: string) {
  const parts = new Intl.DateTimeFormat("en-GB", { timeZone: timezone, hour: "2-digit", minute: "2-digit", hourCycle: "h23" }).formatToParts(value)
  const part = (type: Intl.DateTimeFormatPartTypes) => parts.find((item) => item.type === type)?.value ?? "00"
  return `${part("hour")}:${part("minute")}`
}

export function timeMinutes(value: string) {
  const [hours = 0, minutes = 0] = value.split(":").map(Number)
  return hours * 60 + minutes
}

export function eventsOnDate(events: ScheduleEvent[], date: string, timezone: string) {
  return events.filter((event) => {
    if (event.allDay) return event.startDate <= date && event.endDate >= date
    return zonedDateKey(event.start, timezone) <= date && zonedDateKey(new Date(event.end.getTime() - 1), timezone) >= date
  })
}

export type TimedLayout = { event: ScheduleEvent; start: number; end: number; lane: number; lanes: number }

export function timedLayouts(events: ScheduleEvent[], date: string, timezone: string): TimedLayout[] {
  const segments = events.filter((event) => !event.allDay && zonedDateKey(event.start, timezone) <= date && zonedDateKey(new Date(event.end.getTime() - 1), timezone) >= date)
    .map((event) => {
      const startDate = zonedDateKey(event.start, timezone)
      const endDate = zonedDateKey(new Date(event.end.getTime() - 1), timezone)
      const start = startDate < date ? 0 : timeMinutes(zonedTime(event.start, timezone))
      const end = endDate > date ? minutesPerDay : timeMinutes(zonedTime(new Date(event.end.getTime() - 1), timezone)) + 1
      return { event, start, end: Math.max(start + 20, end), lane: 0, lanes: 1 }
    })
    .sort((left, right) => left.start - right.start || right.end - left.end)

  const laneEnds: number[] = []
  for (const segment of segments) {
    let lane = laneEnds.findIndex((end) => end <= segment.start)
    if (lane < 0) lane = laneEnds.length
    laneEnds[lane] = segment.end
    segment.lane = lane
  }
  for (const segment of segments) segment.lanes = Math.max(1, laneEnds.length)
  return segments
}
