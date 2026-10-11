import { describe, expect, it } from "vitest"
import { scheduleOccurrences2events } from "~/features/schedule/converters/scheduleOccurrences2events"

describe("scheduleOccurrences2events", () => {
  it("keeps all-day calendar dates in the event timezone and converts exclusive end to inclusive", () => {
    const events = scheduleOccurrences2events([{
      id: "occurrence-1",
      blueprint_id: "blueprint-1",
      occurrence_date: "2026-10-10",
      start_at: "2026-10-09T15:00:00Z",
      end_at: "2026-10-11T15:00:00Z",
      timezone: "Asia/Tokyo",
      all_day: true,
      can_update: true,
      can_assign: true,
      persisted: true,
    }], [])

    expect(events).toHaveLength(1)
    expect(events[0].startDate).toBe("2026-10-10")
    expect(events[0].endDate).toBe("2026-10-11")
    expect(events[0].permissions.canAssign).toBe(true)
  })

  it("keeps virtual occurrences without a persisted ID using stable blueprint/date identity", () => {
    const occurrence = {
      blueprint_id: "blueprint with spaces",
      occurrence_date: "2026-10-12",
      start_at: "2026-10-12T09:00:00Z",
      end_at: "2026-10-12T10:00:00Z",
      timezone: "UTC",
      persisted: false,
    }
    const events = scheduleOccurrences2events([occurrence, occurrence], [])
    const [event] = events

    expect(events).toHaveLength(2)
    expect(event.id).toBe("schedule-occurrence:blueprint%20with%20spaces:2026-10-12")
    expect(events[1].id).toBe(event.id)
    expect(event.persisted).toBe(false)
    expect(event.blueprintId).toBe(occurrence.blueprint_id)
    expect(event.occurrenceDate).toBe(occurrence.occurrence_date)
  })
})
