import { describe, expect, it } from "vitest"
import type { RestAllocationWeekResponse } from "~/api/generated/allocations"
import { allocationWeeks2weeklyPlanBoardProps } from "./allocationWeeks2weeklyPlanBoardProps"

describe("allocationWeeks2weeklyPlanBoardProps", () => {
  it("uses backend totals and splits overnight active time and allocations in the user's timezone", () => {
    const week = allocationWeeks2weeklyPlanBoardProps([{
      week_start: "2026-10-12",
      timezone: "Asia/Tokyo",
      target_minutes: 600,
      allocated_minutes: 120,
      difference_minutes: -480,
      free_minutes: 1320,
      projects: [{ project_id: "p1", title: "Research", color_index: 1, target_minutes: 600, allocated_minutes: 120, difference_minutes: -480 }],
      days: [12, 13, 14, 15, 16, 17, 18].map((day) => ({ date: `2026-10-${day}`, active_start_at: "2026-10-11T22:00:00Z", active_end_at: "2026-10-12T14:00:00Z", allocated_minutes: 120, free_minutes: 1320 })),
      allocations: [{
        id: "allocation-1", rule_id: "rule-1", occurrence_date: "2026-10-12",
        project_id: "p1", project_title: "Research", timezone: "Asia/Tokyo",
        start_at: "2026-10-12T14:00:00Z", end_at: "2026-10-12T16:00:00Z",
        revision: 2, rule_revision: 3, can_update: true,
      }, {
        id: "moved-allocation", rule_id: "rule-moved", occurrence_date: "2026-10-12",
        project_id: "p1", project_title: "Research", timezone: "Asia/Tokyo",
        start_at: "2026-10-13T00:00:00Z", end_at: "2026-10-13T01:00:00Z",
        revision: 4, rule_revision: 5, can_update: true,
      }, {
        id: "midnight-allocation", rule_id: "rule-midnight", occurrence_date: "2026-10-12",
        project_id: "p1", project_title: "Research", timezone: "Asia/Tokyo",
        start_at: "2026-10-13T14:00:00Z", end_at: "2026-10-13T15:00:00Z",
        revision: 6, rule_revision: 7, can_update: true,
      }],
      active_intervals: [{ local_date: "2026-10-12", start_at: "2026-10-12T14:00:00Z", end_at: "2026-10-12T22:00:00Z" }],
    } satisfies RestAllocationWeekResponse])[0]!

    expect(week.allocatedMinutes).toBe(120)
    expect(week.differenceMinutes).toBe(-480)
    expect(week.allocations).toEqual(expect.arrayContaining([
      expect.objectContaining({ date: "2026-10-12", startMinute: 1380, endMinute: 1440, ruleId: "rule-1", occurrenceDate: "2026-10-12" }),
      expect.objectContaining({ date: "2026-10-13", startMinute: 0, endMinute: 60, ruleId: "rule-1", occurrenceDate: "2026-10-12" }),
      expect.objectContaining({ date: "2026-10-13", startMinute: 540, endMinute: 600, ruleId: "rule-moved", occurrenceDate: "2026-10-12" }),
      expect.objectContaining({ date: "2026-10-13", startMinute: 1380, endMinute: 1440, ruleId: "rule-midnight" }),
    ]))
    expect(week.activeIntervals).toEqual([
      { date: "2026-10-12", startMinute: 1380, endMinute: 1440 },
      { date: "2026-10-13", startMinute: 0, endMinute: 420 },
    ])
  })

  it("renders moved occurrences by their actual date and clips exact-midnight ends", () => {
    const [week] = allocationWeeks2weeklyPlanBoardProps([{
      week_start: "2026-10-19",
      timezone: "Asia/Tokyo",
      days: Array.from({ length: 7 }, (_, index) => ({ date: `2026-10-${String(index + 19).padStart(2, "0")}` })),
      allocations: [{
        id: "moved-to-next-week", rule_id: "rule-1", occurrence_date: "2026-10-12",
        project_id: "p1", project_title: "Research", start_at: "2026-10-19T14:00:00Z", end_at: "2026-10-19T15:00:00Z",
        revision: 4, rule_revision: 6, can_update: true,
      }],
    } satisfies RestAllocationWeekResponse])

    expect(week?.allocations).toHaveLength(1)
    expect(week?.allocations[0]).toMatchObject({
      date: "2026-10-19", startMinute: 1380, endMinute: 1440,
      occurrenceDate: "2026-10-12", ruleId: "rule-1", startLabel: "23:00", endLabel: "00:00",
    })
  })
})
