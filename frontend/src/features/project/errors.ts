import { ApiError } from "~/api/http"
import type { RestErrResponse } from "~/api/generated/projects"
import type { MessageKey } from "~/features/i18n/hooks"
import type { ProjectFormValues } from "~/features/project/components/ProjectForm/schema"

type ProjectFieldError = Partial<Record<keyof ProjectFormValues, MessageKey>>

export function getProjectFormFieldErrors(error: unknown): ProjectFieldError | undefined {
  if (!(error instanceof ApiError)) return undefined

  const details = (error.data as RestErrResponse | undefined)?.error?.details ?? []
  const fieldErrors: ProjectFieldError = {}
  for (const detail of details) {
    const field = detail.field?.toLowerCase()
    const code = detail.code
    if (field === "title") fieldErrors.title = code === "required" ? "projects.form.requiredTitle" : "projects.form.invalidTitle"
    else if (field === "type") fieldErrors.type = "projects.form.invalidType"
    else if (field === "priority") fieldErrors.priority = "projects.form.invalidPriority"
    else if (field === "start_date") fieldErrors.startDate = "projects.form.invalidDate"
    else if (field === "end_date") fieldErrors.endDate = code === "invalid_date_range" ? "projects.form.dateOrder" : "projects.form.invalidDate"
  }

  return Object.keys(fieldErrors).length > 0 ? fieldErrors : undefined
}

export function getProjectErrorMessageKey(error: unknown): MessageKey {
  if (error instanceof ApiError) {
    const details = (error.data as RestErrResponse | undefined)?.error?.details ?? []
    if (error.status === 401) return "projects.error.unauthorized"
    if (error.status === 403) return "projects.error.forbidden"
    if (error.status === 409 || details.some((detail) => detail.code === "revision_conflict")) return "projects.error.conflict"
  }
  if (error instanceof TypeError) return "projects.error.network"
  return "projects.error.generic"
}
