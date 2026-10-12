import { describe, expect, it } from "vitest"
import type { RestInboxSchedulePayloadResponse } from "~/api/generated/inbox"
import { formatInboxSchedule } from "./formatInboxSchedule"

function schedule(overrides: Partial<RestInboxSchedulePayloadResponse> = {}): RestInboxSchedulePayloadResponse {
  return {
    all_day: true, assignee_id: "user-1", rule_id: "schedule-1", can_assign: true, can_update: true,
    can_update_following: true, completed: false, description: "", end_at: "2026-10-13T15:00:00Z", location: "",
    occurrence_date: "2026-10-13", occurrence_id: null, revision: 1, skipped: false, source_revision_id: "rev-1",
    start_at: "2026-10-12T15:00:00Z", timezone: "Asia/Tokyo", ...overrides,
  }
}

describe("formatInboxSchedule", () => {
  it("shows one-day all-day ranges without the exclusive end date", () => {
    const label = formatInboxSchedule(schedule(), "America/Los_Angeles", "ja-JP", "終日")
    expect(label).toContain("10/13")
    expect(label).not.toContain("10/14")
    expect(label).toContain("終日")
  })

  it("shows inclusive multi-day all-day ranges", () => {
    const label = formatInboxSchedule(schedule({ end_at: "2026-10-15T15:00:00Z" }), "UTC", "ja-JP", "終日")
    expect(label).toContain("10/13")
    expect(label).toContain("10/15")
    expect(label).not.toContain("10/16")
  })

  it("shows timed schedules in the user's timezone", () => {
    const label = formatInboxSchedule(schedule({
      all_day: false,
      start_at: "2026-10-13T00:30:00Z",
      end_at: "2026-10-13T01:30:00Z",
    }), "America/Los_Angeles", "en-US", "All day")
    expect(label).toContain("5:30 PM")
    expect(label).toContain("6:30 PM")
    expect(label).not.toContain("9:30 AM")
  })
})
