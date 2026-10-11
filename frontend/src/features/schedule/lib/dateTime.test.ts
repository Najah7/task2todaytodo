import { describe, expect, it } from "vitest"
import { localDateTimeToInstant, ScheduleDateTimeError } from "~/features/schedule/lib/dateTime"

describe("localDateTimeToInstant", () => {
  it("uses the earlier instant for an ambiguous fall-back time", () => {
    expect(localDateTimeToInstant("2026-11-01T01:30:00", "America/New_York"))
      .toBe("2026-11-01T05:30:00Z")
  })

  it("rejects a nonexistent spring-forward wall time", () => {
    expect(() => localDateTimeToInstant("2026-03-08T02:30:00", "America/New_York"))
      .toThrowError(expect.objectContaining({ code: "nonexistent-local-time" }))
  })

  it("rejects an invalid IANA timezone", () => {
    expect(() => localDateTimeToInstant("2026-06-01T09:00:00", "Mars/Olympus"))
      .toThrowError(expect.objectContaining<Partial<ScheduleDateTimeError>>({ code: "invalid-timezone" }))
  })
})
