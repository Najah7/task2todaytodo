import { Temporal } from "@js-temporal/polyfill"

export function dateAndMinute2localDateTime(date: string, minute: number) {
  const offset = Math.floor(minute / 1440)
  const localMinute = ((minute % 1440) + 1440) % 1440
  const [year = 1970, month = 1, day = 1] = date.split("-").map(Number)
  const targetDate = Temporal.PlainDate.from({ year, month, day }).add({ days: offset })
  const hour = Math.floor(localMinute / 60)
  const minutes = localMinute % 60
  return targetDate.toString() + "T" + String(hour).padStart(2, "0") + ":" + String(minutes).padStart(2, "0")
}
