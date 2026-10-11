import { describe, expect, it } from "vitest"
import { emptyScheduleFormValues, scheduleCreateSchema } from "~/features/schedule/components/ScheduleForm/schema"

describe("scheduleCreateSchema", () => {
  it("requires an end date no earlier than the start date", () => {
    const result = scheduleCreateSchema.safeParse({
      ...emptyScheduleFormValues,
      title: "Planning",
      startDate: "2026-10-11",
      endDate: "2026-10-10",
      timezone: "Asia/Tokyo",
    })
    expect(result.success).toBe(false)
    if (!result.success) expect(result.error.issues.some((issue) => issue.path[0] === "endDate")).toBe(true)
  })

  it("rejects nonexistent local wall times", () => {
    const result = scheduleCreateSchema.safeParse({
      ...emptyScheduleFormValues,
      title: "DST transition",
      startDate: "2026-03-08",
      endDate: "2026-03-08",
      startTime: "02:30",
      endTime: "03:30",
      timezone: "America/New_York",
    })
    expect(result.success).toBe(false)
    if (!result.success) expect(result.error.issues.some((issue) => issue.message === "schedules.form.nonexistentLocalTime")).toBe(true)
  })

  it("accepts an all-day date range and a valid repeat", () => {
    const result = scheduleCreateSchema.safeParse({
      ...emptyScheduleFormValues,
      title: "Vacation",
      startDate: "2026-10-11",
      endDate: "2026-10-12",
      allDay: true,
      timezone: "Asia/Tokyo",
      frequency: "weekly",
      weekdays: ["sun"],
    })
    expect(result.success).toBe(true)
  })

  it("does not constrain an individual moved occurrence by the read-only recurrence end", () => {
    const values = {
      ...emptyScheduleFormValues,
      title: "Moved exception",
      startDate: "2026-10-20",
      endDate: "2026-10-20",
      timezone: "UTC",
      frequency: "daily" as const,
      recurrenceEndDate: "2026-10-15",
      recurrenceAnchorDate: "2026-10-10",
    }
    expect(scheduleCreateSchema.safeParse({ ...values, scope: "current" }).success).toBe(true)
    expect(scheduleCreateSchema.safeParse({ ...values, scope: "following" }).success).toBe(true)
    expect(scheduleCreateSchema.safeParse({ ...values, recurrenceAnchorDate: "2026-10-20", scope: "following" }).success).toBe(false)
  })
})
