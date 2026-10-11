import { z } from "zod"
import { emptyScheduleFormValues, scheduleCreateSchema } from "~/features/schedule/components/ScheduleForm/schema"

export const inboxQuickCreateSchema = z.object({
  title: z.string().trim().min(1, "tasks.form.requiredTitle"),
  projectId: z.string(),
  startDate: z.string(),
  endDate: z.string(),
  startTime: z.string(),
  endTime: z.string(),
  allDay: z.boolean(),
  timezone: z.string(),
}).superRefine((values, context) => {
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
  title: "",
  projectId: "",
  startDate: "",
  endDate: "",
  startTime: "09:00",
  endTime: "10:00",
  allDay: false,
  timezone: "UTC",
}
