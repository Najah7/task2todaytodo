import { describe, expect, it } from "vitest"
import { emptyInboxQuickCreateValues, inboxQuickCreateSchema } from "~/features/inbox/components/InboxQuickCreate/schema"

const valid = {
  ...emptyInboxQuickCreateValues,
  formType: "schedule" as const,
  title: "Planning",
  startDate: "2026-10-12",
  endDate: "2026-10-12",
  timezone: "Asia/Tokyo",
}

describe("inboxQuickCreateSchema", () => {
  it("accepts a timed one-off schedule with the user's timezone", () => {
    expect(inboxQuickCreateSchema.safeParse(valid).success).toBe(true)
  })

  it("accepts inclusive multi-day all-day schedules", () => {
    expect(inboxQuickCreateSchema.safeParse({ ...valid, endDate: "2026-10-15", allDay: true }).success).toBe(true)
  })

  it("requires a title, timezone, and non-reversed date range", () => {
    expect(inboxQuickCreateSchema.safeParse({ ...valid, title: " " }).success).toBe(false)
    expect(inboxQuickCreateSchema.safeParse({ ...valid, timezone: " " }).success).toBe(false)
    expect(inboxQuickCreateSchema.safeParse({ ...valid, endDate: "2026-10-11" }).success).toBe(false)
  })

  it("rejects invalid and nonexistent local schedule times", () => {
    expect(inboxQuickCreateSchema.safeParse({ ...valid, startTime: "25:00" }).success).toBe(false)
    expect(inboxQuickCreateSchema.safeParse({
      ...valid,
      startDate: "2026-03-08",
      endDate: "2026-03-08",
      startTime: "02:30",
      timezone: "America/New_York",
    }).success).toBe(false)
  })
})
