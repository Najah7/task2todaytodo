import type { FormEventHandler } from "react"
import type { MessageKey } from "~/features/i18n/messages/types"

export type ProjectFormOption = { value: string; label: string }

export type ProjectFormConfirmation = {
  open: boolean
  onConfirm: () => void
  onCancel: () => void
}

export type ProjectFormProps = {
  heading: MessageKey
  submitLabel: MessageKey
  options: { types: ProjectFormOption[]; priorities: ProjectFormOption[] }
  onSubmit: FormEventHandler<HTMLFormElement>
  onCancel: () => void
  discardConfirmation: ProjectFormConfirmation
  conflict?: {
    onRequestReload: () => void
    reloading: boolean
    confirmation: ProjectFormConfirmation
  }
}
