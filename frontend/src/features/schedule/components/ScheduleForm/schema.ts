import { z } from "zod"
import { localDateTimeText, localDateTimeToInstant, ScheduleDateTimeError } from "~/features/schedule/lib/dateTime"

const datePattern = /^\d{4}-\d{2}-\d{2}$/
const timePattern = /^(?:[01]\d|2[0-3]):[0-5]\d$/

export const scheduleCreateSchema = z.object({
  title: z.string().trim().min(1, "schedules.form.requiredTitle"),
  startDate: z.string().regex(datePattern, "schedules.form.invalidDate"),
  endDate: z.string().regex(datePattern, "schedules.form.invalidDate"),
  startTime: z.string(),
  endTime: z.string(),
  allDay: z.boolean(),
  timezone: z.string().trim().min(1, "schedules.form.requiredTimezone"),
  frequency: z.enum(["once", "daily", "weekly", "monthly", "yearly"]),
  interval: z.number().int().positive("schedules.form.invalidInterval"),
  weekdays: z.array(z.string()),
  recurrenceEndDate: z.string(),
  recurrenceAnchorDate: z.string(),
  projectId: z.string(),
  assigneeId: z.string(),
  location: z.string(),
  description: z.string(),
  scope: z.enum(["current", "following"]),
}).superRefine((values, context) => {
  if (values.startDate && values.endDate && values.startDate > values.endDate) {
    context.addIssue({ code: "custom", path: ["endDate"], message: "schedules.form.endBeforeStart" })
  }
  if (!values.allDay) {
    if (!timePattern.test(values.startTime)) {
      context.addIssue({ code: "custom", path: ["startTime"], message: "schedules.form.invalidTime" })
    }
    if (!timePattern.test(values.endTime)) {
      context.addIssue({ code: "custom", path: ["endTime"], message: "schedules.form.invalidTime" })
    }
    if (`${values.startDate}T${values.startTime}` >= `${values.endDate}T${values.endTime}`) {
      context.addIssue({ code: "custom", path: ["endTime"], message: "schedules.form.endBeforeStart" })
    }
    for (const [field, date, time] of [
      ["startTime", values.startDate, values.startTime],
      ["endTime", values.endDate, values.endTime],
    ] as const) {
      if (!datePattern.test(date) || !timePattern.test(time) || !values.timezone) continue
      try {
        localDateTimeToInstant(localDateTimeText(date, time), values.timezone)
      } catch (error) {
        if (error instanceof ScheduleDateTimeError && error.code === "nonexistent-local-time") {
          context.addIssue({ code: "custom", path: [field], message: "schedules.form.nonexistentLocalTime" })
        } else if (error instanceof ScheduleDateTimeError && error.code === "invalid-timezone") {
          context.addIssue({ code: "custom", path: ["timezone"], message: "schedules.form.invalidTimezone" })
        }
      }
    }
  }
  if (values.frequency === "weekly" && values.weekdays.length === 0) {
    context.addIssue({ code: "custom", path: ["weekdays"], message: "schedules.form.requiredWeekday" })
  }
  const recurrenceStart = values.scope === "following" && values.recurrenceAnchorDate ? values.recurrenceAnchorDate : values.startDate
  if (values.scope === "following" && values.frequency !== "once" && values.recurrenceEndDate && values.recurrenceEndDate < recurrenceStart) {
    context.addIssue({ code: "custom", path: ["recurrenceEndDate"], message: "schedules.form.endBeforeStart" })
  }
})

export type ScheduleFormValues = z.infer<typeof scheduleCreateSchema>

export const emptyScheduleFormValues: ScheduleFormValues = {
  title: "",
  startDate: "",
  endDate: "",
  startTime: "09:00",
  endTime: "10:00",
  allDay: false,
  timezone: "UTC",
  frequency: "once",
  interval: 1,
  weekdays: [],
  recurrenceEndDate: "",
  recurrenceAnchorDate: "",
  projectId: "",
  assigneeId: "",
  location: "",
  description: "",
  scope: "current",
}
