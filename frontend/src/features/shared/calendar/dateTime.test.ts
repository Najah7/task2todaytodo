import { describe, expect, it } from "vitest"
import { instantToLocalDateTime, localDateTimeToInstant, ScheduleDateTimeError } from "~/features/shared/calendar/dateTime"

describe("localDateTimeToInstant", () => {
  it("converts wall time using the requested timezone", () => {
    expect(localDateTimeToInstant("2026-10-13T08:45", "Asia/Tokyo")).toBe("2026-10-12T23:45:00Z")
    expect(instantToLocalDateTime("2026-10-12T23:45:00Z", "Asia/Tokyo")).toBe("2026-10-13T08:45:00")
  })

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
