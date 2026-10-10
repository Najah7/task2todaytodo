import { createContext } from "react"
import type { MessageKey } from "~/features/i18n/messages/types"
import type { TaskFormDirtyFields } from "~/features/task/converters/taskFormValues2updateRequest"
import type { TaskFormValues } from "~/features/task/components/TaskForm/schema"

export type TaskFormSaveResult = {
  complete: boolean
  taskSaved?: boolean
  taskResetValues?: Partial<Pick<TaskFormValues, "title" | "description" | "dueDate" | "priority" | "projectId" | "manualEstimate">>
  actionItems?: {
    index: number
    key: string
    error?: MessageKey
    fieldErrors?: Partial<Record<"title" | "estimatedMinutes" | "priority", MessageKey>>
    deletionSucceeded?: boolean
    identity?: { seriesId: string; occurrenceDate: string }
    savedFields?: Partial<Pick<TaskFormValues["actionItems"][number], "title" | "estimatedMinutes" | "priority">>
  }[]
  revisionRefreshError?: boolean
}

export type TaskFormReloadSnapshot = { values: TaskFormValues }

export type TaskFormSubmissionError = Error & {
  conflict?: true
  fieldErrors?: Record<string, MessageKey>
}

export type UpdateActionItemCompletionInput = {
  taskId: string
  actionItemId: string
  seriesId?: string
  occurrenceDate?: string
  completed: boolean
}

export type TaskActions = {
  busy: boolean
  submit: (values: TaskFormValues, dirtyFields: TaskFormDirtyFields) => Promise<TaskFormSaveResult>
  saveComplete: () => void
  retry: () => Promise<void>
  retryOptions: () => Promise<void>
  reloadLatest?: () => Promise<TaskFormReloadSnapshot>
  updateActionItemCompletion: (input: UpdateActionItemCompletionInput) => Promise<void>
}

export const TaskActionContext = createContext<TaskActions | null>(null)
