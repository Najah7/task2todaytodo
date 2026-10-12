import type { RestScheduleRuleCreateRequest } from "~/api/generated/schedules"
import { addCalendarDays } from "~/features/shared/calendar/dateTime"
import type { ScheduleFormValues } from "~/features/schedule/components/ScheduleForm/schema"

export function scheduleFormValues2ruleCreateRequests(values: ScheduleFormValues[]): RestScheduleRuleCreateRequest[] {
  return values.map((item) => {
    const once = item.frequency === "once"
    const endDate = item.allDay ? addCalendarDays(item.endDate, 1) : item.endDate
    return {
      title: item.title.trim(),
      description: item.description,
      location: item.location,
      project_id: item.projectId || "",
      assignee_id: item.assigneeId || "",
      timezone: item.timezone,
      all_day: item.allDay,
      local_start: `${item.startDate}T${item.allDay ? "00:00:00" : `${item.startTime}:00`}`,
      local_end: `${endDate}T${item.allDay ? "00:00:00" : `${item.endTime}:00`}`,
      frequency: item.frequency,
      interval: once ? 0 : item.interval,
      weekdays: item.frequency === "weekly" ? item.weekdays : [],
      ...(once || !item.recurrenceEndDate ? {} : { end_date: item.recurrenceEndDate }),
    }
  })
}
