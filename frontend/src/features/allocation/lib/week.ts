const dayMilliseconds = 86_400_000

function parseDate(date: string) {
  const [year, month, day] = date.split("-").map(Number)
  return new Date(Date.UTC(year ?? 1970, (month ?? 1) - 1, day ?? 1))
}

export function dateKey(date: Date) {
  return `${date.getUTCFullYear()}-${String(date.getUTCMonth() + 1).padStart(2, "0")}-${String(date.getUTCDate()).padStart(2, "0")}`
}

export function mondayOf(date: string) {
  const monday = parseDate(date)
  monday.setUTCDate(monday.getUTCDate() - ((monday.getUTCDay() + 6) % 7))
  return dateKey(monday)
}

export function weekDates(weekStart: string) {
  const monday = parseDate(weekStart)
  return Array.from({ length: 7 }, (_, index) => {
    const date = new Date(monday)
    date.setUTCDate(date.getUTCDate() + index)
    return dateKey(date)
  })
}

export function shiftWeek(weekStart: string, amount: number) {
  const date = parseDate(mondayOf(weekStart))
  date.setTime(date.getTime() + amount * 7 * dayMilliseconds)
  return dateKey(date)
}

export function snapMinuteToQuarter(minute: number) {
  return Math.max(0, Math.min(24 * 60, Math.round(minute / 15) * 15))
}

export function dateInTimezone(value: Date, timezone: string) {
  const parts = new Intl.DateTimeFormat("en-CA", { timeZone: timezone, year: "numeric", month: "2-digit", day: "2-digit" }).formatToParts(value)
  const part = (type: Intl.DateTimeFormatPartTypes) => parts.find((item) => item.type === type)?.value ?? "00"
  return part("year") + "-" + part("month") + "-" + part("day")
}

export function isDateKey(value: string | null): value is string {
  if (!value || !/^\d{4}-\d{2}-\d{2}$/.test(value)) return false
  const date = parseDate(value)
  return !Number.isNaN(date.getTime()) && dateKey(date) === value
}
