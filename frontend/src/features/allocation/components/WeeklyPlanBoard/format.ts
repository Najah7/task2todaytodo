export function durationLabel(minutes: number) {
  const hours = Math.floor(Math.abs(minutes) / 60)
  const remainder = Math.abs(minutes) % 60
  return `${hours}:${String(remainder).padStart(2, "0")}`
}

export function localDateTime(date: string, minute: number) {
  const dayOffset = Math.floor(minute / 1440)
  const timeMinute = ((minute % 1440) + 1440) % 1440
  const [year = 1970, month = 1, day = 1] = date.split("-").map(Number)
  const dateValue = new Date(Date.UTC(year, month - 1, day + dayOffset))
  const datePart = `${dateValue.getUTCFullYear()}-${String(dateValue.getUTCMonth() + 1).padStart(2, "0")}-${String(dateValue.getUTCDate()).padStart(2, "0")}`
  return `${datePart}T${String(Math.floor(timeMinute / 60)).padStart(2, "0")}:${String(timeMinute % 60).padStart(2, "0")}`
}

export function timeMinute(time: string) {
  const [hour = 0, minute = 0] = time.split(":").map(Number)
  return hour * 60 + minute
}
