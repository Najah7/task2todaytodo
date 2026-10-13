import { Temporal } from "@js-temporal/polyfill"
import type { RestAllocationWeekResponse } from "~/api/generated/allocations"
import { zonedTime } from "~/features/shared/calendar/zonedTime"
import { instantToLocalDateTime } from "~/features/shared/calendar/dateTime"
import type { WeeklyPlanActiveIntervalView, WeeklyPlanBoardProps } from "~/features/allocation/components/WeeklyPlanBoard/types"

type WeeklyPlanBoardModel = Omit<WeeklyPlanBoardProps,
  "canEdit" | "busy" | "onChangeWeek" | "onEditPlan" | "onCreateAllocation" | "onEditAllocation" | "onEditActiveTime"
>
function splitActiveInterval(startAt: string, endAt: string, timezone: string): WeeklyPlanActiveIntervalView[] {
  const end = Temporal.Instant.from(endAt).toZonedDateTimeISO(timezone)
  let cursor = Temporal.Instant.from(startAt).toZonedDateTimeISO(timezone)
  const segments: WeeklyPlanActiveIntervalView[] = []
  while (Temporal.ZonedDateTime.compare(cursor, end) < 0) {
    const nextDay = cursor.startOfDay().add({ days: 1 })
    const segmentEnd = Temporal.ZonedDateTime.compare(end, nextDay) < 0 ? end : nextDay
    const endsAtMidnight = Temporal.ZonedDateTime.compare(segmentEnd, nextDay) === 0
    const endMinute = endsAtMidnight ? 1440 : segmentEnd.hour * 60 + segmentEnd.minute
    const startMinute = cursor.hour * 60 + cursor.minute
    if (endMinute > startMinute) segments.push({ date: cursor.toPlainDate().toString(), startMinute, endMinute })
    cursor = segmentEnd
  }
  return segments
}

export function allocationWeeks2weeklyPlanBoardProps(weeks: RestAllocationWeekResponse[]): WeeklyPlanBoardModel[] {
  return weeks.map((week) => {
    const timezone = week.timezone || "UTC"
    const projects = (week.projects ?? []).map((project, index) => ({
      id: project.project_id || "project-" + index,
      title: project.title || "",
      targetMinutes: project.target_minutes ?? 0,
      allocatedMinutes: project.allocated_minutes ?? 0,
      differenceMinutes: project.difference_minutes ?? 0,
      colorIndex: project.color_index ?? index,
    }))
    const colors = new Map(projects.map((project) => [project.id, project.colorIndex]))
    const days = (week.days ?? []).map((day) => ({
      date: day.date || "",
      allocatedMinutes: day.allocated_minutes ?? 0,
      freeMinutes: day.free_minutes ?? 0,
      activeStartAt: day.active_start_at ? instantToLocalDateTime(day.active_start_at, timezone).slice(11, 16) : "",
      activeEndAt: day.active_end_at ? instantToLocalDateTime(day.active_end_at, timezone).slice(11, 16) : "",
      hasActiveException: day.has_active_exception ?? false,
      activeRevision: day.active_revision ?? 0,
    }))
    const weekDates = new Set(days.map((day) => day.date))
    const allocations = (week.allocations ?? []).filter((item) => item.rule_id && item.occurrence_date && item.start_at && item.end_at).flatMap((item) => {
      const end = Temporal.Instant.from(item.end_at!).toZonedDateTimeISO(timezone)
      let cursor = Temporal.Instant.from(item.start_at!).toZonedDateTimeISO(timezone)
      const result = []
      while (Temporal.ZonedDateTime.compare(cursor, end) < 0) {
        const nextDay = cursor.startOfDay().add({ days: 1 })
        const segmentEnd = Temporal.ZonedDateTime.compare(end, nextDay) < 0 ? end : nextDay
        const endsAtMidnight = Temporal.ZonedDateTime.compare(segmentEnd, nextDay) === 0
        const date = cursor.toPlainDate().toString()
        if (weekDates.has(date)) {
          result.push({
            id: (item.id || item.rule_id + ":" + item.occurrence_date) + ":" + date,
            ruleId: item.rule_id!,
            occurrenceDate: item.occurrence_date!,
            revision: item.revision ?? 0,
            ruleRevision: item.rule_revision ?? 0,
            timezone,
            startAt: instantToLocalDateTime(item.start_at!, timezone).slice(0, 16),
            endAt: instantToLocalDateTime(item.end_at!, timezone).slice(0, 16),
            projectId: item.project_id || "",
            projectTitle: item.project_title || "",
            colorIndex: colors.get(item.project_id || "") ?? 0,
            date,
            startMinute: cursor.hour * 60 + cursor.minute,
            endMinute: endsAtMidnight ? 1440 : segmentEnd.hour * 60 + segmentEnd.minute,
            startLabel: zonedTime(new Date(cursor.toInstant().epochMilliseconds), timezone),
            endLabel: zonedTime(new Date(segmentEnd.toInstant().epochMilliseconds), timezone),
            canUpdate: item.can_update ?? false,
            canDelete: item.can_delete ?? false,
            canUpdateFollowing: item.can_update_following ?? false,
            canDeleteFollowing: item.can_delete_following ?? false,
          })
        }
        cursor = segmentEnd
      }
      return result
    })
    const activeIntervals = (week.active_intervals ?? []).flatMap((active) =>
      active.start_at && active.end_at ? splitActiveInterval(active.start_at, active.end_at, timezone) : [],
    )
    const titles = new Map(projects.map((project) => [project.id, project.title]))
    const pausedRules = (week.paused_rules ?? []).filter((rule) => rule.rule_id).map((rule) => ({
      ruleId: rule.rule_id!,
      projectId: rule.project_id || "",
      projectTitle: titles.get(rule.project_id || "") || "",
      reason: rule.reason || "",
    }))
    return {
      weekStart: week.week_start || "",
      timezone,
      projects,
      days,
      allocations,
      activeIntervals,
      pausedRules,
      allocatedMinutes: week.allocated_minutes ?? 0,
      targetMinutes: week.target_minutes ?? 0,
      differenceMinutes: week.difference_minutes ?? 0,
      freeMinutes: week.free_minutes ?? 0,
    }
  })
}
