import { Temporal } from "@js-temporal/polyfill"
import { localDateStartInstant, plainDate } from "~/features/schedule/lib/dateTime"
import type { ScheduleView } from "~/features/schedule/providers/ScheduleNavigationProvider/context"

export type ScheduleVisibleRange = {
  startDate: string
  endDateExclusive: string
  from: string
  to: string
}

export function scheduleRange(anchor: string, view: ScheduleView, timezone: string): ScheduleVisibleRange {
  const date = plainDate(anchor)
  let start: Temporal.PlainDate
  let end: Temporal.PlainDate
  if (view === "day") {
    start = date
    end = date.add({ days: 1 })
  } else if (view === "week") {
    start = date.subtract({ days: date.dayOfWeek - 1 })
    end = start.add({ days: 7 })
  } else {
    const monthStart = date.with({ day: 1 })
    start = monthStart.subtract({ days: monthStart.dayOfWeek - 1 })
    const nextMonth = monthStart.add({ months: 1 })
    end = nextMonth.add({ days: (8 - nextMonth.dayOfWeek) % 7 })
  }
  // Two calendar days cover all supported IANA offsets and floating all-day dates.
  const paddedStart = start.subtract({ days: 2 }).toString()
  const paddedEnd = end.add({ days: 2 }).toString()
  return {
    startDate: start.toString(),
    endDateExclusive: end.toString(),
    from: localDateStartInstant(paddedStart, timezone).toString(),
    to: localDateStartInstant(paddedEnd, timezone).toString(),
  }
}

export function scheduleEventOverlapsRange(
  event: { start: Date; end: Date; allDay: boolean; startDate: string; endDate: string; timezone: string },
  range: Pick<ScheduleVisibleRange, "startDate" | "endDateExclusive">,
  displayTimezone: string,
): boolean {
  if (event.allDay) {
    return event.startDate < range.endDateExclusive && event.endDate >= range.startDate
  }
  const visibleStart = localDateStartInstant(range.startDate, displayTimezone).epochMilliseconds
  const visibleEnd = localDateStartInstant(range.endDateExclusive, displayTimezone).epochMilliseconds
  return event.start.getTime() < visibleEnd && event.end.getTime() > visibleStart
}
