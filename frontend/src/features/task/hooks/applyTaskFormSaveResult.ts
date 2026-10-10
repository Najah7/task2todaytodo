import type { Dispatch, SetStateAction } from "react"
import type { UseFormReturn } from "react-hook-form"
import type { MessageKey } from "~/features/i18n/messages/types"
import type { TaskFormSaveResult } from "~/features/task/providers/TaskActionProvider/context"
import type { TaskFormValues } from "~/features/task/components/TaskForm/schema"

export function applyTaskFormSaveResult(
  form: UseFormReturn<TaskFormValues>,
  result: TaskFormSaveResult,
  setActionItemErrors: Dispatch<SetStateAction<Record<string, MessageKey>>>,
): void {
  for (const [name, value] of Object.entries(result.taskResetValues ?? {})) {
    form.resetField(name as "title" | "description" | "dueDate" | "priority" | "projectId" | "manualEstimate", { defaultValue: value as never })
  }
  for (const item of result.actionItems ?? []) {
    setActionItemErrors((current) => {
      const next = { ...current }
      if (item.error && !Object.keys(item.fieldErrors ?? {}).length) next[item.key] = item.error
      else delete next[item.key]
      return next
    })
    for (const [field, message] of Object.entries(item.fieldErrors ?? {})) {
      form.setError(`actionItems.${item.index}.${field}` as `actionItems.${number}.title`, { type: "server", message })
    }
    if (item.identity) {
      form.setValue(`actionItems.${item.index}.seriesId`, item.identity.seriesId, { shouldDirty: false })
      form.setValue(`actionItems.${item.index}.occurrenceDate`, item.identity.occurrenceDate, { shouldDirty: false })
    }
    if (item.savedFields) {
      for (const [field, value] of Object.entries(item.savedFields)) {
        form.resetField(`actionItems.${item.index}.${field}` as `actionItems.${number}.${"title" | "estimatedMinutes" | "priority"}`, { defaultValue: value as never })
      }
    }
    if (item.deletionSucceeded) {
      form.setValue(`actionItems.${item.index}.deletionSucceeded`, true, { shouldDirty: false })
    }
  }
}

export function setTaskServerFieldErrors(form: UseFormReturn<TaskFormValues>, errors: Record<string, unknown>): boolean {
  let unknownField = false
  for (const [path, message] of Object.entries(errors)) {
    const formPath = serverFieldPath(path)
    if (formPath) {
      form.setError(formPath, {
        type: "server",
        message: typeof message === "string" ? message : "tasks.form.invalidField",
      })
    } else unknownField = true
  }
  return unknownField
}

function serverFieldPath(path: string): "title" | "projectId" | "dueDate" | "description" | "priority" | "manualEstimate" | `actionItems.${number}.${"title" | "estimatedMinutes" | "priority"}` | undefined {
  const fields = { title: "title", project_id: "projectId", due_date: "dueDate", description: "description", priority: "priority", manual_estimated_minutes: "manualEstimate" } as const
  if (path in fields) return fields[path as keyof typeof fields]
  const actionItemMatch = path.match(/^action_items\.(\d+)\.(title|estimated_minutes|priority)$/)
  if (!actionItemMatch) return undefined
  const field = actionItemMatch[2] === "estimated_minutes" ? "estimatedMinutes" : actionItemMatch[2]
  return `actionItems.${actionItemMatch[1]}.${field}` as `actionItems.${number}.${"title" | "estimatedMinutes" | "priority"}`
}
