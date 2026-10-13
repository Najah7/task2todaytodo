import type { FormEventHandler } from "react"
import type { UseFormReturn } from "react-hook-form"
import type { MessageKey } from "~/features/i18n/messages/types"
import type { TaskFormValues } from "./schema"

export type TaskFormOption = { value: string; label: string }
export type TaskEstimateSource = "manual" | "action_items"

export type TaskFormConfirmation = {
  open: boolean
  onConfirm: () => void
  onCancel: () => void
}

export type TaskFormConflict = {
  onRequestReload: () => void
  reloading: boolean
  confirmation: TaskFormConfirmation
}

export type TaskFormProps = {
  form: UseFormReturn<TaskFormValues>
  heading: MessageKey
  submitLabel: MessageKey
  options: { projects: TaskFormOption[]; priorities: TaskFormOption[] }
  estimateSource?: TaskEstimateSource
  showTaskPriorityInheritance: boolean
  showActionItemPriorityInheritance: boolean
  showEstimateWillRecalculate: boolean
  partialSaveMessage?: MessageKey
  actionItemErrors: Record<string, MessageKey>
  serverError: boolean
  onClearActionItemError: (key: string) => void
  onSubmit: FormEventHandler<HTMLFormElement>
  onCancel: () => void
  discardConfirmation: TaskFormConfirmation
  conflict?: TaskFormConflict
}

export type TaskFormController = {
  form: UseFormReturn<TaskFormValues>
  viewProps: Omit<TaskFormProps, "form">
}
