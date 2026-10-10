import { z } from "zod"

const optionalEstimate = z.string().refine((value) => {
  if (!value.trim()) return true
  if (!/^\d+(?::[0-5]\d)?$/.test(value.trim())) return false
  const [first, second] = value.trim().split(":")
  const minutes = second === undefined ? Number(first) : Number(first) * 60 + Number(second)
  return Number.isSafeInteger(minutes) && minutes >= 0
}, "tasks.form.invalidEstimate")

export const taskCreateSchema = z.object({
  title: z.string().trim().min(1, "tasks.form.requiredTitle"),
  projectId: z.string(),
  dueDate: z.string(),
  description: z.string(),
  priority: z.string(),
  manualEstimate: optionalEstimate,
  actionItems: z.array(z.object({
    clientKey: z.string().optional(),
    seriesId: z.string().optional(),
    occurrenceDate: z.string().optional(),
    isRecurring: z.boolean().optional(),
    completed: z.boolean().optional(),
    pendingDelete: z.boolean().optional(),
    deletionSucceeded: z.boolean().optional(),
    title: z.string().trim(),
    estimatedMinutes: z.string(),
    priority: z.string(),
  }).superRefine((item, context) => {
    if (item.pendingDelete || item.deletionSucceeded) return
    if (!item.title) {
      context.addIssue({ code: "custom", path: ["title"], message: "tasks.form.requiredActionItemTitle" })
    }
    if (!optionalEstimate.safeParse(item.estimatedMinutes).success) {
      context.addIssue({ code: "custom", path: ["estimatedMinutes"], message: "tasks.form.invalidEstimate" })
    }
  })),
})

export type TaskFormValues = z.infer<typeof taskCreateSchema>
export type TaskCreateFormValues = TaskFormValues

export const emptyTaskFormValues: TaskFormValues = {
  title: "",
  projectId: "",
  dueDate: "",
  description: "",
  priority: "",
  manualEstimate: "",
  actionItems: [],
}

export const emptyTaskCreateFormValues = emptyTaskFormValues

export function durationTextToMinutes(value: string): number | undefined {
  const text = value.trim()
  if (!text || !/^\d+(?::[0-5]\d)?$/.test(text)) return undefined
  const [hours, minutes = "0"] = text.split(":")
  const total = text.includes(":") ? Number(hours) * 60 + Number(minutes) : Number(hours)
  return Number.isSafeInteger(total) ? total : undefined
}

export function minutesToDurationText(value: number | null | undefined): string {
  if (value == null || !Number.isSafeInteger(value) || value < 0) return ""
  const hours = Math.floor(value / 60)
  const minutes = value % 60
  return `${hours}:${String(minutes).padStart(2, "0")}`
}
