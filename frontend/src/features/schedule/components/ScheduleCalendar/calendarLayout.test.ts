import { describe, expect, it } from "vitest"
import type { ScheduleEvent } from "~/features/schedule/providers/ScheduleAPIDataProvider/context"
import { calendarDates, dateKey, eventsOnDate, formatDate, timedLayouts } from "./calendarLayout"

function event(overrides: Partial<ScheduleEvent>): ScheduleEvent {
  return {
    id: "01J00000000000000000000000",
    blueprintId: "01J00000000000000000000001",
    occurrenceDate: "2026-10-13",
    title: "Review",
    description: "",
    location: "",
    start: new Date("2026-10-13T16:00:00Z"),
    end: new Date("2026-10-13T17:00:00Z"),
    localStart: "2026-10-13T09:00:00",
    localEnd: "2026-10-13T10:00:00",
    startDate: "2026-10-13",
    endDate: "2026-10-13",
    timezone: "America/Los_Angeles",
    allDay: false,
    projectId: "",
    projectTitle: "",
    assigneeId: "",
    revision: 1,
    blueprintRevision: 1,
    state: "scheduled",
    persisted: true,
    permissions: { canUpdate: true, canDelete: true, canComplete: true, canSkip: true, canRestore: false, canUpdateFollowing: true, canDeleteFollowing: true, canAssign: true },
    ...overrides,
  }
}

describe("calendarLayout", () => {
  it("uses the exact date span of each visible range", () => {
    const week = calendarDates("2026-10-13", "week", "America/Los_Angeles")
    expect(week.map(dateKey)).toEqual([
      "2026-10-12", "2026-10-13", "2026-10-14", "2026-10-15", "2026-10-16", "2026-10-17", "2026-10-18",
    ])

    const month = calendarDates("2026-10-13", "month", "America/Los_Angeles")
    expect(month[0] && dateKey(month[0])).toBe("2026-09-28")
    expect(month.at(-1) && dateKey(month.at(-1)!)).toBe("2026-11-01")
    expect(month).toHaveLength(35)
  })

  it("formats date-only labels in UTC regardless of browser timezone", () => {
    const value = new Date("2026-10-13T00:00:00Z")
    expect(formatDate(value, "en", { dateStyle: "full" })).toBe("Tuesday, October 13, 2026")
    expect(formatDate(value, "ja", { dateStyle: "medium" })).toBe("2026/10/13")
  })

  it("keeps all-day spans on their declared calendar dates", () => {
    const allDay = event({ allDay: true, startDate: "2026-10-12", endDate: "2026-10-14", start: new Date("2026-10-11T15:00:00Z"), end: new Date("2026-10-14T15:00:00Z") })
    expect(eventsOnDate([allDay], "2026-10-12", "America/Los_Angeles")).toEqual([allDay])
    expect(eventsOnDate([allDay], "2026-10-14", "Asia/Tokyo")).toEqual([allDay])
    expect(eventsOnDate([allDay], "2026-10-15", "UTC")).toEqual([])
  })

  it("splits overnight events and assigns separate lanes to overlaps", () => {
    const first = event({ start: new Date("2026-10-13T16:00:00Z"), end: new Date("2026-10-13T18:00:00Z") })
    const second = event({ id: "01J00000000000000000000002", start: new Date("2026-10-13T17:00:00Z"), end: new Date("2026-10-13T19:00:00Z") })
    const after = event({ id: "01J00000000000000000000003", start: new Date("2026-10-13T18:00:00Z"), end: new Date("2026-10-13T19:00:00Z") })
    const overnight = event({ id: "01J00000000000000000000004", start: new Date("2026-10-14T05:00:00Z"), end: new Date("2026-10-14T09:00:00Z") })
    const layouts = timedLayouts([first, second, after, overnight], "2026-10-13", "America/Los_Angeles")
    expect(layouts.slice(0, 3).map(({ lane, lanes }) => [lane, lanes])).toEqual([[0, 2], [1, 2], [0, 2]])
    expect(layouts[3]).toMatchObject({ start: 22 * 60, end: 24 * 60 })
    expect(timedLayouts([overnight], "2026-10-14", "America/Los_Angeles")[0]).toMatchObject({ start: 0, end: 120 })
  })
})
