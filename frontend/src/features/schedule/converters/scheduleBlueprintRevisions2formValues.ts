import type { ScheduleFormValues } from "~/features/schedule/components/ScheduleForm/schema"
import { addCalendarDays } from "~/features/schedule/lib/dateTime"
import type { ScheduleBlueprintRevision } from "~/features/schedule/providers/ScheduleAPIDataProvider/context"

export function scheduleBlueprintRevisions2formValues(
  revisions: ScheduleBlueprintRevision[],
  selectedOccurrenceDate?: string,
): ScheduleFormValues[] {
  return revisions.map((revision) => {
    const startDate = selectedOccurrenceDate ?? revision.localStart.slice(0, 10)
    const sourceStartDate = revision.localStart.slice(0, 10)
    const sourceEndDate = revision.localEnd.slice(0, 10)
    const duration = Math.max(0, Math.round((Date.parse(`${sourceEndDate}T00:00:00Z`) - Date.parse(`${sourceStartDate}T00:00:00Z`)) / 86_400_000))
    const endDate = addCalendarDays(startDate, Math.max(0, duration - (revision.allDay ? 1 : 0)))
    return {
      title: revision.title,
      startDate,
      endDate,
      startTime: revision.allDay ? "00:00" : revision.localStart.slice(11, 16),
      endTime: revision.allDay ? "00:00" : revision.localEnd.slice(11, 16),
      allDay: revision.allDay,
      timezone: revision.timezone,
      frequency: revision.frequency,
      interval: revision.interval,
      weekdays: [...revision.weekdays],
      recurrenceEndDate: revision.endDate,
      recurrenceAnchorDate: selectedOccurrenceDate ?? startDate,
      projectId: revision.projectId,
      assigneeId: revision.assigneeId,
      location: revision.location,
      description: revision.description,
      scope: "following" as const,
    }
  })
}
