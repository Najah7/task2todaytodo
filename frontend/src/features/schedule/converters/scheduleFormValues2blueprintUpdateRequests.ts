import type { RestScheduleBlueprintUpdateRequest } from "~/api/generated/schedules"
import { scheduleFormValues2blueprintCreateRequests } from "~/features/schedule/converters/scheduleFormValues2blueprintCreateRequests"
import type { ScheduleFormValues } from "~/features/schedule/components/ScheduleForm/schema"

export function scheduleFormValues2blueprintUpdateRequests(values: ScheduleFormValues[], baselines: ScheduleFormValues[] = []): RestScheduleBlueprintUpdateRequest[] {
  return values.map((item, index) => {
    const baseline = baselines[index]
    const createRequest = { ...scheduleFormValues2blueprintCreateRequests([item])[0] }
    delete createRequest.assignee_id
    const request: RestScheduleBlueprintUpdateRequest = { occurrence_date: item.recurrenceAnchorDate, scope: "following" }
    if (!baseline || item.title !== baseline.title) request.title = item.title.trim()
    if (!baseline || item.description !== baseline.description) request.description = item.description
    if (!baseline || item.location !== baseline.location) request.location = item.location
    if (!baseline || item.projectId !== baseline.projectId) request.project_id = item.projectId || null
    if (!baseline || item.timezone !== baseline.timezone) request.timezone = item.timezone
    if (!baseline || item.allDay !== baseline.allDay) request.all_day = item.allDay
    if (!baseline || item.frequency !== baseline.frequency || item.interval !== baseline.interval || !sameValues(item.weekdays, baseline.weekdays) || item.recurrenceEndDate !== baseline.recurrenceEndDate) {
      request.frequency = createRequest.frequency
      request.interval = createRequest.interval
      request.weekdays = createRequest.weekdays
      request.end_date = item.frequency === "once" || !item.recurrenceEndDate ? null : item.recurrenceEndDate
    }
    if (startChanged(item, baseline)) request.local_start = createRequest.local_start
    if (endChanged(item, baseline)) request.local_end = createRequest.local_end
    if ((!baseline || item.assigneeId !== baseline.assigneeId) && item.assigneeId) request.assignee_id = item.assigneeId
    return request
  })
}

function startChanged(item: ScheduleFormValues, baseline?: ScheduleFormValues): boolean {
  if (!baseline) return true
  return item.allDay !== baseline.allDay || item.timezone !== baseline.timezone
    || item.startDate !== baseline.startDate || item.startTime !== baseline.startTime
}

function endChanged(item: ScheduleFormValues, baseline?: ScheduleFormValues): boolean {
  if (!baseline) return true
  return item.allDay !== baseline.allDay || item.timezone !== baseline.timezone
    || item.endDate !== baseline.endDate || item.endTime !== baseline.endTime
}

function sameValues(left: string[], right: string[]): boolean {
  return left.length === right.length && left.every((value, index) => value === right[index])
}
