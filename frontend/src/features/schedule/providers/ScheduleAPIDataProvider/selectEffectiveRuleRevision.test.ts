import { describe, expect, it } from "vitest"
import { selectEffectiveRuleRevision } from "~/features/schedule/providers/ScheduleAPIDataProvider/selectEffectiveRuleRevision"
import type { ScheduleRuleRevision } from "~/features/schedule/providers/ScheduleAPIDataProvider/context"

function revision(overrides: Partial<ScheduleRuleRevision>): ScheduleRuleRevision {
  return {
    revisionId: "rev-1", revision: 1, effectiveFrom: "2026-01-01", superseded: false, deleted: false,
    title: "Event", description: "", location: "", projectId: "", assigneeId: "user", timezone: "UTC",
    allDay: false, localStart: "2026-01-01T09:00:00", localEnd: "2026-01-01T10:00:00", frequency: "daily",
    interval: 1, weekdays: [], endDate: "", ...overrides,
  }
}

describe("selectEffectiveRuleRevision", () => {
  it("uses an occurrence's effective source revision", () => {
    const initial = revision({ revisionId: "initial", effectiveUntil: "2026-06-30" })
    const future = revision({ revisionId: "future", revision: 2, effectiveFrom: "2026-07-01", timezone: "Asia/Tokyo" })
    expect(selectEffectiveRuleRevision([initial, future], "2026-08-01", "future")).toBe(future)
  })

  it("does not reuse a superseded source for a saved future exception", () => {
    const superseded = revision({ revisionId: "source", superseded: true, timezone: "America/New_York" })
    const current = revision({ revisionId: "current", revision: 3, effectiveFrom: "2026-07-01", timezone: "Asia/Tokyo" })
    expect(selectEffectiveRuleRevision([superseded, current], "2026-08-01", "source")).toBe(current)
  })

  it("does not apply a revision outside the selected occurrence date", () => {
    const future = revision({ revisionId: "future", effectiveFrom: "2026-09-01" })
    expect(selectEffectiveRuleRevision([future], "2026-08-01", "future")).toBeUndefined()
  })
})
