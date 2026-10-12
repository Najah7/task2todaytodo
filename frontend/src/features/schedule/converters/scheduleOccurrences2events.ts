import type { RestScheduleOccurrenceResponse } from "~/api/generated/schedules"
import { addCalendarDays, instantToLocalDate, instantToLocalDateTime } from "~/features/shared/calendar/dateTime"
import type { ScheduleEvent, ScheduleProjectOption } from "~/features/schedule/providers/ScheduleAPIDataProvider/context"

export function scheduleOccurrences2events(
  occurrences: RestScheduleOccurrenceResponse[],
  projectOptions: ScheduleProjectOption[],
): ScheduleEvent[] {
  return occurrences.flatMap((occurrence) => {
    if (!occurrence.rule_id || !occurrence.occurrence_date || !occurrence.start_at || !occurrence.end_at) return []
    const timezone = occurrence.timezone || "UTC"
    const start = new Date(occurrence.start_at)
    const end = new Date(occurrence.end_at)
    if (!Number.isFinite(start.getTime()) || !Number.isFinite(end.getTime())) return []
    const allDay = occurrence.all_day === true
    const startDate = instantToLocalDate(occurrence.start_at, timezone)
    const exclusiveEndDate = instantToLocalDate(occurrence.end_at, timezone)
    const endDate = allDay ? addCalendarDays(exclusiveEndDate, -1) : exclusiveEndDate
    const projectId = occurrence.project_id || ""
    const projectTitle = projectOptions.find((project) => project.id === projectId)?.title || ""
    return [{
      id: occurrence.id || `schedule-occurrence:${encodeURIComponent(occurrence.rule_id)}:${occurrence.occurrence_date}`,
      ruleId: occurrence.rule_id,
      occurrenceDate: occurrence.occurrence_date,
      title: occurrence.title || "",
      description: occurrence.description || "",
      location: occurrence.location || "",
      start,
      end,
      localStart: instantToLocalDateTime(occurrence.start_at, timezone),
      localEnd: instantToLocalDateTime(occurrence.end_at, timezone),
      startDate,
      endDate,
      timezone,
      allDay,
      projectId,
      projectTitle,
      assigneeId: occurrence.assignee_id || "",
      sourceRevisionId: occurrence.source_revision_id,
      revision: occurrence.revision || 0,
      state: occurrence.skipped ? "skipped" as const : occurrence.completed ? "completed" as const : "scheduled" as const,
      persisted: occurrence.persisted === true,
      permissions: {
        canUpdate: occurrence.can_update === true,
        canDelete: occurrence.can_delete === true,
        canComplete: occurrence.can_update === true,
        canSkip: occurrence.can_skip === true,
        canRestore: occurrence.can_skip === true && occurrence.skipped === true,
        canAssign: occurrence.can_assign === true,
        canUpdateFollowing: occurrence.can_update_following === true,
        canDeleteFollowing: occurrence.can_delete_following === true,
      },
    }]
  })
}
