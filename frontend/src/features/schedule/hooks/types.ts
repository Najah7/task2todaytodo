import type { BaseSyntheticEvent } from "react"
import type { UseFormReturn } from "react-hook-form"
import type { MessageKey } from "~/features/i18n/messages/types"
import type { ScheduleFormValues } from "~/features/schedule/components/ScheduleForm/schema"
import type { ScheduleAssigneeOption, ScheduleProjectOption } from "~/features/schedule/providers/ScheduleAPIDataProvider/context"

export type ScheduleFormViewProps = {
  heading: MessageKey
  submitLabel: MessageKey
  projectOptions: ScheduleProjectOption[]
  assigneeOptions: ScheduleAssigneeOption[]
  timezoneOptions: string[]
  frequencyOptions: ScheduleFormValues["frequency"][]
  showScopeSelector: boolean
  showRecurrenceFields: boolean
  recurrenceReadOnly: boolean
  timezoneReadOnly: boolean
  canEditContent: boolean
  canAssign: boolean
  onProjectChange: (projectId: string) => void
  serverError?: MessageKey
  hasConflict: boolean
  reloading: boolean
  onRequestReload: () => void
  reloadConfirmation: { open: boolean; onConfirm: () => void; onCancel: () => void }
  onSubmit: (event?: BaseSyntheticEvent) => Promise<void>
  onCancel: () => void
  onReloadLatest: () => void
  discardConfirmation: { open: boolean; onConfirm: () => void; onCancel: () => void }
}

export type ScheduleFormController = {
  form: UseFormReturn<ScheduleFormValues>
  viewProps: ScheduleFormViewProps
}
