import type { ScheduleFormValues } from "~/features/schedule/components/ScheduleForm/schema"
import type { ScheduleBlueprintRevision, ScheduleEvent } from "~/features/schedule/providers/ScheduleAPIDataProvider/context"

export function scheduleEvents2formValues(
  events: ScheduleEvent[],
  revisions: ScheduleBlueprintRevision[] = [],
): ScheduleFormValues[] {
  return events.map((event) => {
    const revision = revisions.find((item) => item.revisionId === event.sourceRevisionId)
      ?? revisions.find((item) => item.effectiveFrom <= event.occurrenceDate && (!item.effectiveUntil || event.occurrenceDate <= item.effectiveUntil))
    const recurrence = event.recurrence
    return {
      title: event.title,
      startDate: event.startDate,
      endDate: event.endDate,
      startTime: event.allDay ? "00:00" : event.localStart.slice(11, 16),
      endTime: event.allDay ? "00:00" : event.localEnd.slice(11, 16),
      allDay: event.allDay,
      timezone: event.timezone,
      frequency: recurrence?.frequency ?? revision?.frequency ?? "once",
      interval: recurrence?.interval ?? revision?.interval ?? 1,
      weekdays: [...(recurrence?.weekdays ?? revision?.weekdays ?? [])],
      recurrenceEndDate: recurrence?.endDate ?? revision?.endDate ?? "",
      recurrenceAnchorDate: event.occurrenceDate,
      projectId: event.projectId,
      assigneeId: event.assigneeId,
      location: event.location,
      description: event.description,
      scope: "current" as const,
    }
  })
}
