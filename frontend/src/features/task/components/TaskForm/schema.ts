import { z } from "zod"

const optionalEstimate = z.string().refine((value) => {
  if (!value.trim()) return true
  if (!/^\d+(?::[0-5]\d)?$/.test(value.trim())) return false
  const [first, second] = value.trim().split(":")
  const minutes = second === undefined ? Number(first) : Number(first) * 60 + Number(second)
    return Number.isSafeInteger(minutes) && minutes >= 0 && minutes <= 2_147_483_647
}, "tasks.form.invalidEstimate")

const requiredActionItemEstimate = z.string().refine((value) => {
  const minutes = durationTextToMinutes(value)
  return minutes !== undefined && minutes >= 1
}, "tasks.form.invalidEstimate")

export const taskCreateSchema = z.object({
  title: z.string().trim().min(1, "tasks.form.requiredTitle"),
  projectId: z.string(),
  dueDate: z.string(),
  description: z.string(),
  priority: z.string(),
  manualEstimate: optionalEstimate,
  taskCompleted: z.boolean().optional(),
  taskStarted: z.boolean().optional(),
  taskEstimateEditable: z.boolean().optional(),
  canAddActionItem: z.boolean().optional(),
  actualMinutes: z.string().optional(),
  actionItems: z.array(z.object({
    clientKey: z.string().optional(),
    id: z.string().optional(),
    completed: z.boolean().optional(),
    started: z.boolean().optional(),
    estimateEditable: z.boolean().optional(),
    actualMinutes: z.string().optional(),
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
    if (!requiredActionItemEstimate.safeParse(item.estimatedMinutes).success) {
      context.addIssue({ code: "custom", path: ["estimatedMinutes"], message: "tasks.form.invalidEstimate" })
    }
    const actualMinutes = item.actualMinutes?.trim()
    if (item.completed && actualMinutes && (!/^\d+$/.test(actualMinutes) || !Number.isSafeInteger(Number(actualMinutes)) || Number(actualMinutes) > 2_147_483_647)) {
      context.addIssue({ code: "custom", path: ["actualMinutes"], message: "tasks.form.invalidActualMinutes" })
    }
  })),
}).superRefine((values, context) => {
  const taskActualMinutes = values.actualMinutes?.trim()
  if (values.taskCompleted && taskActualMinutes && (!/^\d+$/.test(taskActualMinutes) || !Number.isSafeInteger(Number(taskActualMinutes)) || Number(taskActualMinutes) > 2_147_483_647)) {
    context.addIssue({ code: "custom", path: ["actualMinutes"], message: "tasks.form.invalidActualMinutes" })
  }
  const activeItems = values.actionItems.filter((item) => !item.pendingDelete && !item.deletionSucceeded)
  if (activeItems.length > 0) {
    const childEstimate = activeItems.reduce((sum, item) => sum + (durationTextToMinutes(item.estimatedMinutes) ?? 0), 0)
    if (childEstimate > 2_147_483_647) {
      context.addIssue({ code: "custom", path: ["manualEstimate"], message: "tasks.form.invalidEstimate" })
    }
    return
  }
  const manualEstimate = durationTextToMinutes(values.manualEstimate)
  if (manualEstimate === undefined || manualEstimate < 1) {
    context.addIssue({ code: "custom", path: ["manualEstimate"], message: "tasks.form.invalidEstimate" })
  }
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
  taskCompleted: false,
  taskStarted: false,
  taskEstimateEditable: true,
  canAddActionItem: true,
  actualMinutes: "",
  actionItems: [],
}

export const emptyTaskCreateFormValues = emptyTaskFormValues

export function durationTextToMinutes(value: string): number | undefined {
  const text = value.trim()
  if (!text || !/^\d+(?::[0-5]\d)?$/.test(text)) return undefined
  const [hours, minutes = "0"] = text.split(":")
  const total = text.includes(":") ? Number(hours) * 60 + Number(minutes) : Number(hours)
  return Number.isSafeInteger(total) && total <= 2_147_483_647 ? total : undefined
}

export function minutesToDurationText(value: number | null | undefined): string {
  if (value == null || !Number.isSafeInteger(value) || value < 0) return ""
  const hours = Math.floor(value / 60)
  const minutes = value % 60
  return `${hours}:${String(minutes).padStart(2, "0")}`
}
