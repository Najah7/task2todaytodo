import { describe, expect, it } from "vitest"
import { scheduleEventOverlapsRange, scheduleRange } from "~/features/schedule/lib/scheduleRange"

describe("scheduleRange", () => {
  it("uses Monday through the following Monday for week views", () => {
    const range = scheduleRange("2026-10-11", "week", "Asia/Tokyo")
    expect(range.startDate).toBe("2026-10-05")
    expect(range.endDateExclusive).toBe("2026-10-12")
    expect(range.from).toBe("2026-10-02T15:00:00Z")
    expect(range.to).toBe("2026-10-13T15:00:00Z")
  })

  it("pads month grids and keeps a full Monday-first week", () => {
    const range = scheduleRange("2026-02-03", "month", "UTC")
    expect(range.startDate).toBe("2026-01-26")
    expect(range.endDateExclusive).toBe("2026-03-02")
  })

  it("filters all-day events using their own local dates", () => {
    const range = { startDate: "2026-10-05", endDateExclusive: "2026-10-12" }
    expect(scheduleEventOverlapsRange({ start: new Date(0), end: new Date(0), allDay: true, startDate: "2026-10-11", endDate: "2026-10-12", timezone: "Pacific/Kiritimati" }, range, "America/Los_Angeles")).toBe(true)
  })
})
