import { emptyScheduleFormValues, type ScheduleFormValues } from "~/features/schedule/components/ScheduleForm/schema"
import type { InboxQuickCreateValues } from "~/features/inbox/components/InboxQuickCreate/schema"

export function inboxQuickCreateValues2scheduleFormValues(values: InboxQuickCreateValues[], assigneeIds: string[]): ScheduleFormValues[] {
  if (values.length !== assigneeIds.length) throw new Error("Schedule values and assignees must have the same length")
  return values.map((value, index) => ({
    ...emptyScheduleFormValues,
    title: value.title,
    projectId: value.projectId,
    startDate: value.startDate,
    endDate: value.endDate,
    startTime: value.startTime,
    endTime: value.endTime,
    allDay: value.allDay,
    timezone: value.timezone,
    frequency: "once",
    interval: 1,
    weekdays: [],
    recurrenceEndDate: "",
    recurrenceAnchorDate: value.startDate,
    assigneeId: assigneeIds[index]!,
    scope: "current",
  }))
}
