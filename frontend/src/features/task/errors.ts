import { ApiError } from "~/api/http"
import type { RestErrResponse } from "~/api/generated/tasks"

export function taskFieldErrors(error: unknown): Record<string, unknown> | undefined {
  if (!(error instanceof ApiError)) return undefined
  if (error.status !== 400) return undefined
  const details = (error.data as RestErrResponse | undefined)?.error?.details ?? []
  if (!details.length) return undefined
  const mapped: Record<string, unknown> = {}
  for (const detail of details) {
    const field = detail.field
    if (field) mapped[field] = true
  }
  return Object.keys(mapped).length ? mapped : undefined
}

export function actionItemFieldErrors(error: unknown): Partial<Record<"title" | "estimatedMinutes" | "priority", "tasks.form.requiredActionItemTitle" | "tasks.form.invalidEstimate" | "tasks.form.invalidPriority" | "tasks.form.invalidField">> | undefined {
  if (!(error instanceof ApiError) || error.status !== 400) return undefined
  const details = (error.data as RestErrResponse | undefined)?.error?.details ?? []
  const fields: Partial<Record<"title" | "estimatedMinutes" | "priority", "tasks.form.requiredActionItemTitle" | "tasks.form.invalidEstimate" | "tasks.form.invalidPriority" | "tasks.form.invalidField">> = {}
  for (const detail of details) {
    if (detail.field === "title") fields.title = detail.code === "title_required" ? "tasks.form.requiredActionItemTitle" : "tasks.form.invalidField"
    else if (detail.field === "estimated_minutes") fields.estimatedMinutes = detail.code === "invalid_minutes" ? "tasks.form.invalidEstimate" : "tasks.form.invalidField"
    else if (detail.field === "priority") fields.priority = detail.code === "invalid_priority" ? "tasks.form.invalidPriority" : "tasks.form.invalidField"
  }
  return Object.keys(fields).length ? fields : undefined
}

export function taskErrorMessageKey(error: unknown): "tasks.form.forbidden" | "tasks.form.conflict" | "tasks.form.networkError" | "tasks.form.genericError" {
  if (error instanceof ApiError) {
    if (error.status === 403) return "tasks.form.forbidden"
    if (error.status === 409) return "tasks.form.conflict"
    if (error.status === 401) return "tasks.form.forbidden"
  }
  if (error instanceof TypeError) return "tasks.form.networkError"
  return "tasks.form.genericError"
}
