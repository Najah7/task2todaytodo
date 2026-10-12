import { getScheduleRulesIdRevisions, type RestScheduleRuleRevisionResponse } from "~/api/generated/schedules"
import type { ScheduleRuleRevision } from "./context"

export async function fetchScheduleRuleRevisions(ruleId: string, signal?: AbortSignal): Promise<ScheduleRuleRevision[]> {
  const revisions: RestScheduleRuleRevisionResponse[] = []
  let pageToken: string | undefined
  do {
    const response = await getScheduleRulesIdRevisions(
      ruleId,
      { page_size: 100, page_token: pageToken },
      signal ? { signal } : undefined,
    )
    revisions.push(...(response.items || []))
    pageToken = response.next_page_token || undefined
  } while (pageToken)
  return revisions.map((revision) => ({
    revisionId: revision.revision_id || "",
    revision: revision.revision || 0,
    effectiveFrom: revision.effective_from || "",
    effectiveUntil: revision.effective_until,
    superseded: revision.superseded === true,
    deleted: revision.deleted === true,
    title: revision.title || "",
    description: revision.description || "",
    location: revision.location || "",
    projectId: revision.project_id || "",
    assigneeId: revision.assignee_id || "",
    timezone: revision.timezone || "UTC",
    allDay: revision.all_day === true,
    localStart: revision.local_start || "",
    localEnd: revision.local_end || "",
    frequency: (revision.frequency || "once") as ScheduleRuleRevision["frequency"],
    interval: revision.interval || 1,
    weekdays: revision.weekdays || [],
    endDate: revision.end_date || "",
  }))
}
