import { Temporal } from "@js-temporal/polyfill"

export type ScheduleDateTimeErrorCode = "invalid-date" | "invalid-time" | "invalid-timezone" | "nonexistent-local-time"

export class ScheduleDateTimeError extends Error {
  readonly code: ScheduleDateTimeErrorCode

  constructor(code: ScheduleDateTimeErrorCode) {
    super(code)
    this.name = "ScheduleDateTimeError"
    this.code = code
  }
}

export function plainDate(value: string): Temporal.PlainDate {
  try {
    return Temporal.PlainDate.from(value, { overflow: "reject" })
  } catch {
    throw new ScheduleDateTimeError("invalid-date")
  }
}

export function localDateTimeToInstant(value: string, timezone: string): string {
  let local: Temporal.PlainDateTime
  try {
    local = Temporal.PlainDateTime.from(value, { overflow: "reject" })
  } catch {
    throw new ScheduleDateTimeError(value.includes("T") ? "invalid-time" : "invalid-date")
  }

  let earlier: Temporal.ZonedDateTime
  let later: Temporal.ZonedDateTime
  try {
    earlier = local.toZonedDateTime(timezone, { disambiguation: "earlier" })
    later = local.toZonedDateTime(timezone, { disambiguation: "later" })
  } catch {
    throw new ScheduleDateTimeError("invalid-timezone")
  }

  if (!earlier.toPlainDateTime().equals(local) && !later.toPlainDateTime().equals(local)) {
    throw new ScheduleDateTimeError("nonexistent-local-time")
  }
  // When the wall time occurs twice, the backend selects the earlier instant.
  return earlier.toInstant().toString()
}

export function localDateTimeText(date: string, time: string): string {
  try {
    return Temporal.PlainDateTime.from(`${date}T${time}:00`, { overflow: "reject" })
      .toString({ smallestUnit: "second" })
  } catch {
    throw new ScheduleDateTimeError(time ? "invalid-time" : "invalid-date")
  }
}

export function localDateStartInstant(date: string, timezone: string): Temporal.Instant {
  const day = plainDate(date)
  try {
    return day.toPlainDateTime("00:00").toZonedDateTime(timezone, { disambiguation: "compatible" }).toInstant()
  } catch {
    throw new ScheduleDateTimeError("invalid-timezone")
  }
}

export function instantToLocalDate(value: string, timezone: string): string {
  try {
    return Temporal.Instant.from(value).toZonedDateTimeISO(timezone).toPlainDate().toString()
  } catch {
    throw new ScheduleDateTimeError("invalid-timezone")
  }
}

export function instantToLocalDateTime(value: string, timezone: string): string {
  try {
    return Temporal.Instant.from(value).toZonedDateTimeISO(timezone).toPlainDateTime().toString({ smallestUnit: "second" })
  } catch {
    throw new ScheduleDateTimeError("invalid-timezone")
  }
}

export function addCalendarDays(date: string, days: number): string {
  return plainDate(date).add({ days }).toString()
}
