import type { RestScheduleOccurrenceUpdateRequest } from "~/api/generated/schedules"
import { addCalendarDays, localDateTimeText, localDateTimeToInstant } from "~/features/shared/calendar/dateTime"
import type { ScheduleFormValues } from "~/features/schedule/components/ScheduleForm/schema"

export function scheduleFormValues2occurrenceUpdateRequests(values: ScheduleFormValues[], baselines: ScheduleFormValues[] = []): RestScheduleOccurrenceUpdateRequest[] {
  return values.map((item, index) => {
    const baseline = baselines[index]
    const endDate = item.allDay ? addCalendarDays(item.endDate, 1) : item.endDate
    const request: RestScheduleOccurrenceUpdateRequest = {}
    if (!baseline || item.title !== baseline.title) request.title = item.title.trim()
    if (!baseline || item.description !== baseline.description) request.description = item.description
    if (!baseline || item.location !== baseline.location) request.location = item.location
    if (!baseline || item.projectId !== baseline.projectId) request.project_id = item.projectId || null
    if (!baseline || item.allDay !== baseline.allDay) request.all_day = item.allDay
    if (startChanged(item, baseline)) {
      request.start_at = localDateTimeToInstant(localDateTimeText(item.startDate, item.allDay ? "00:00" : item.startTime), item.timezone)
    }
    if (endChanged(item, baseline)) {
      request.end_at = localDateTimeToInstant(localDateTimeText(endDate, item.allDay ? "00:00" : item.endTime), item.timezone)
    }
    if ((!baseline || item.assigneeId !== baseline.assigneeId) && item.assigneeId) request.assignee_id = item.assigneeId
    return request
  })
}

function startChanged(item: ScheduleFormValues, baseline?: ScheduleFormValues): boolean {
  return !baseline || item.allDay !== baseline.allDay || item.timezone !== baseline.timezone
    || item.startDate !== baseline.startDate || item.startTime !== baseline.startTime
}

function endChanged(item: ScheduleFormValues, baseline?: ScheduleFormValues): boolean {
  return !baseline || item.allDay !== baseline.allDay || item.timezone !== baseline.timezone
    || item.endDate !== baseline.endDate || item.endTime !== baseline.endTime
}
