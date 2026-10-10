import type { MessageKey } from "~/features/i18n/types"

export function getValidationMessageKey(message: string): MessageKey {
  switch (message) {
    case "auth.error.invalidEmail":
    case "auth.error.invalidPassword":
    case "auth.error.emailExists":
    case "auth.error.passwordMismatch":
      return message
    default:
      return "auth.error.generic"
  }
}
