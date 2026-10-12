import { describe, expect, it } from "vitest"
import { emptyScheduleFormValues, type ScheduleFormValues } from "~/features/schedule/components/ScheduleForm/schema"
import { scheduleFormValues2ruleCreateRequests } from "~/features/schedule/converters/scheduleFormValues2ruleCreateRequests"
import { scheduleFormValues2ruleUpdateRequests } from "~/features/schedule/converters/scheduleFormValues2ruleUpdateRequests"
import { scheduleFormValues2occurrenceUpdateRequests } from "~/features/schedule/converters/scheduleFormValues2occurrenceUpdateRequests"

const values: ScheduleFormValues = {
  ...emptyScheduleFormValues,
  title: "Focus block",
  startDate: "2026-10-10",
  endDate: "2026-10-11",
  timezone: "Asia/Tokyo",
}

describe("schedule form request converters", () => {
  it("omits one-off recurrence fields and uses interval zero", () => {
    const [request] = scheduleFormValues2ruleCreateRequests([values])
    expect(request).toMatchObject({ frequency: "once", interval: 0, weekdays: [], local_start: "2026-10-10T09:00:00", local_end: "2026-10-11T10:00:00" })
    expect(request).not.toHaveProperty("end_date")
  })

  it("makes all-day end exclusive in local calendar dates for rule and occurrence requests", () => {
    const allDay = { ...values, allDay: true }
    const [rule] = scheduleFormValues2ruleCreateRequests([allDay])
    const [occurrence] = scheduleFormValues2occurrenceUpdateRequests([allDay])
    expect(rule.local_start).toBe("2026-10-10T00:00:00")
    expect(rule.local_end).toBe("2026-10-12T00:00:00")
    expect(occurrence.start_at).toBe("2026-10-09T15:00:00Z")
    expect(occurrence.end_at).toBe("2026-10-11T15:00:00Z")
  })

  it("clears a prior recurrence end and Project when following updates make them unlimited and personal", () => {
    const [request] = scheduleFormValues2ruleUpdateRequests(
      [{ ...values, frequency: "daily", recurrenceEndDate: "" }],
      [{ ...values, projectId: "project-1", frequency: "daily", recurrenceEndDate: "2026-12-01" }],
    )
    expect(request).toMatchObject({ interval: 1, project_id: null, end_date: null, weekdays: [] })
    expect(request).not.toHaveProperty("assignee_id")
  })

  it("encodes a once-to-weekly following change as interval one with selected weekdays", () => {
    const [request] = scheduleFormValues2ruleUpdateRequests(
      [{ ...values, frequency: "weekly", interval: 1, weekdays: ["MO", "WE"] }],
      [{ ...values, frequency: "once", interval: 1, weekdays: [] }],
    )
    expect(request).toMatchObject({ frequency: "weekly", interval: 1, weekdays: ["MO", "WE"] })
  })

  it("omits unchanged datetime fields from metadata-only edits to preserve stored precision and DST-fold instants", () => {
    const baseline = { ...values, startTime: "01:30", endTime: "02:30", timezone: "America/New_York" }
    const edited = { ...baseline, title: "Updated title" }
    const [occurrence] = scheduleFormValues2occurrenceUpdateRequests([edited], [baseline])
    const [rule] = scheduleFormValues2ruleUpdateRequests([edited], [baseline])
    expect(occurrence).not.toHaveProperty("start_at")
    expect(occurrence).not.toHaveProperty("end_at")
    expect(rule).not.toHaveProperty("local_start")
    expect(rule).not.toHaveProperty("local_end")
  })
})
