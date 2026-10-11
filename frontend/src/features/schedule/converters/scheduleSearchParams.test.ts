import { describe, expect, it } from "vitest"
import { scheduleSearchParams2state, scheduleState2searchParams } from "~/features/schedule/converters/scheduleSearchParams"

describe("schedule URL state", () => {
  it("defaults to a Monday-first week and validates the date", () => {
    expect(scheduleSearchParams2state(new URLSearchParams("view=year&date=2026-02-30"), "2026-10-11"))
      .toEqual({ date: "2026-10-11", view: "week", projectId: undefined, showSkipped: false })
  })

  it("retains calendar filters while dropping stale pagination", () => {
    const previous = new URLSearchParams("sort=title&page_token=old")
    const next = scheduleState2searchParams({ date: "2026-10-11", view: "month", projectId: "p1", showSkipped: true }, previous)
    expect(next.get("date")).toBe("2026-10-11")
    expect(next.get("view")).toBe("month")
    expect(next.get("project_id")).toBe("p1")
    expect(next.get("show_skipped")).toBe("true")
    expect(next.get("page_token")).toBeNull()
    expect(next.get("sort")).toBe("title")
  })
})
