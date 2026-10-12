import { describe, expect, it } from "vitest"
import { dateAndMinute2localDateTime } from "./localDateTimes"

describe("allocation local date and time conversions", () => {
  it("keeps overnight end times on following date", () => {
    expect(dateAndMinute2localDateTime("2026-10-12", 1500)).toBe("2026-10-13T01:00")
  })
})
