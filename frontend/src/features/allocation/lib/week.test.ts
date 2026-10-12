import { describe, expect, it } from "vitest"
import { mondayOf, shiftWeek, snapMinuteToQuarter, weekDates } from "./week"

describe("weekly plan date controls", () => {
  it("normalizes dates to Monday and advances across month boundaries", () => {
    expect(mondayOf("2026-10-18")).toBe("2026-10-12")
    expect(shiftWeek("2026-10-26", 1)).toBe("2026-11-02")
    expect(weekDates("2026-10-12")).toEqual([
      "2026-10-12", "2026-10-13", "2026-10-14", "2026-10-15", "2026-10-16", "2026-10-17", "2026-10-18",
    ])
  })

  it("snaps drag boundaries to quarter hours while allowing end of day", () => {
    expect(snapMinuteToQuarter(127)).toBe(120)
    expect(snapMinuteToQuarter(128)).toBe(135)
    expect(snapMinuteToQuarter(1442)).toBe(1440)
  })
})
