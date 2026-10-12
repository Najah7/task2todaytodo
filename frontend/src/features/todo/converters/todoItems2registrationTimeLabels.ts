import type { RestTodoItemResponse } from "~/api/generated/todo"

export function todoItems2registrationTimeLabels(items: RestTodoItemResponse[] | undefined, timezone: string, locale: string): Map<string, string> {
  const formatter = new Intl.DateTimeFormat(locale, { timeZone: timezone || "UTC", hour: "2-digit", minute: "2-digit", hourCycle: "h23" })
  const labels = new Map<string, string>()
  for (const item of items ?? []) {
    const sourceId = item.kind === "task" ? item.task_id : item.kind === "action_item" ? item.action_item_id : undefined
    if (!sourceId || item.history || labels.has(sourceId) || !item.scheduled_start) continue
    const date = new Date(item.scheduled_start)
    if (!Number.isNaN(date.getTime())) labels.set(sourceId, formatter.format(date))
  }
  return labels
}
