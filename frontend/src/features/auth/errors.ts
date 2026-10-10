import type { MessageKey } from "~/features/i18n/messages/types"
import type { RestErrResponse } from "~/api/generated/auth"
import { ApiError } from "~/api/http"

export class SessionError extends Error {
  readonly translationKey: MessageKey

  constructor(translationKey: MessageKey) {
    super(translationKey)
    this.translationKey = translationKey
  }
}

export function getAuthFieldError(error: unknown): { field: "email" | "password"; translationKey: MessageKey } | null {
  if (!(error instanceof ApiError)) return null
  const details = (error.data as RestErrResponse | undefined)?.error?.details ?? []
  for (const detail of details) {
    if (detail.code === "invalid_email") return { field: "email", translationKey: "auth.error.invalidEmail" }
    if (detail.code === "email_already_exists") return { field: "email", translationKey: "auth.error.emailExists" }
    if (detail.code === "invalid_password") return { field: "password", translationKey: "auth.error.invalidPassword" }
  }
  return null
}

export function getAuthErrorMessage(error: unknown): MessageKey {
  if (error instanceof SessionError) return error.translationKey
  if (error instanceof ApiError) {
    const data = error.data as RestErrResponse | undefined
    const codes = data?.error?.details?.map((detail) => detail.code) ?? []
    if (codes.includes("email_already_exists") || error.status === 409) {
      return "auth.error.emailExists"
    }
    if (codes.includes("invalid_email")) return "auth.error.invalidEmail"
    if (codes.includes("invalid_password")) {
      return "auth.error.invalidPassword"
    }
    if (codes.includes("invalid_credentials") || error.status === 401) {
      return "auth.error.invalidCredentials"
    }
    if (error.status >= 500) return "auth.error.server"
  }
  if (error instanceof TypeError) return "auth.error.connection"
  return "auth.error.generic"
}
