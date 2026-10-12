import { z } from "zod"
import { emptyScheduleFormValues, scheduleCreateSchema } from "~/features/schedule/components/ScheduleForm/schema"

export const inboxQuickCreateSchema = z.object({
  formType: z.enum(["task", "schedule"]),
  title: z.string().trim().min(1, "tasks.form.requiredTitle"),
  projectId: z.string(),
  taskEstimate: z.string(),
  startDate: z.string(),
  endDate: z.string(),
  startTime: z.string(),
  endTime: z.string(),
  allDay: z.boolean(),
  timezone: z.string(),
}).superRefine((values, context) => {
  if (values.formType === "task") {
    const estimate = values.taskEstimate.trim()
    if (!/^\d+(?::[0-5]\d)?$/.test(estimate)) {
      context.addIssue({ code: "custom", path: ["taskEstimate"], message: "tasks.form.invalidEstimate" })
    } else {
      const [hours, minutes = "0"] = estimate.split(":")
      const total = estimate.includes(":") ? Number(hours) * 60 + Number(minutes) : Number(hours)
      if (!Number.isSafeInteger(total) || total < 1 || total > 2_147_483_647) {
        context.addIssue({ code: "custom", path: ["taskEstimate"], message: "tasks.form.invalidEstimate" })
      }
    }
    return
  }
  const schedule = scheduleCreateSchema.safeParse({
    ...emptyScheduleFormValues,
    ...values,
    frequency: "once",
    interval: 1,
    weekdays: [],
    recurrenceEndDate: "",
    recurrenceAnchorDate: values.startDate,
    assigneeId: "",
    location: "",
    description: "",
    scope: "current",
  })
  if (schedule.success) return
  for (const issue of schedule.error.issues) {
    if (issue.path[0] === "title" || issue.path[0] === "projectId") continue
    context.addIssue({ ...issue, path: issue.path })
  }
})

export type InboxQuickCreateValues = z.infer<typeof inboxQuickCreateSchema>

export const emptyInboxQuickCreateValues: InboxQuickCreateValues = {
  formType: "task",
  title: "",
  projectId: "",
  taskEstimate: "",
  startDate: "",
  endDate: "",
  startTime: "09:00",
  endTime: "10:00",
  allDay: false,
  timezone: "UTC",
}
