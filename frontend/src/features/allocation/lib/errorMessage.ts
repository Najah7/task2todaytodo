import type { MessageKey } from "~/features/i18n/hooks"

type Translate = (key: MessageKey, params?: Record<string, string | number>) => string

function asRecord(value: unknown): Record<string, unknown> | undefined {
  return value !== null && typeof value === "object" ? value as Record<string, unknown> : undefined
}

function conflictDetails(error: unknown) {
  const response = asRecord(asRecord(error)?.data)
  const body = asRecord(response?.error)
  const details = Array.isArray(body?.details) ? body.details : []
  for (const detailValue of details) {
    const detail = asRecord(detailValue)
    const metadata = asRecord(detail?.metadata)
    if (metadata && (metadata.kind === "allocation_conflict" || metadata.kind === "outside_active_time")) return metadata
  }
  return undefined
}

function localDateTime(value: unknown, timezone: string, language: string) {
  if (typeof value !== "string") return ""
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ""
  const locale = language === "ja" ? "ja-JP" : "en-US"
  return new Intl.DateTimeFormat(locale, {
    timeZone: timezone,
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    hourCycle: "h23",
  }).format(date)
}

export function allocationErrorMessage(
  error: unknown,
  translate: Translate,
  options: { status?: number; timezone?: string; language?: string; projectNames?: Map<string, string> } = {},
) {
  if (options.status === 412) return translate("weeklyPlan.error.revision")
  const details = conflictDetails(error)
  if (options.status === 409 && details) {
    const projectId = typeof details.project_id === "string" ? details.project_id : ""
    const project = options.projectNames?.get(projectId) || translate("weeklyPlan.unknownProject")
    const start = localDateTime(details.start_at, options.timezone || "UTC", options.language || "en")
    const end = localDateTime(details.end_at, options.timezone || "UTC", options.language || "en")
    return translate(details.kind === "outside_active_time" ? "weeklyPlan.error.outsideActiveTime" : "weeklyPlan.error.allocationOverlap", { project, start, end })
  }
  if (options.status === 409) return translate("weeklyPlan.error.conflict")
  return translate("weeklyPlan.error.request")
}
