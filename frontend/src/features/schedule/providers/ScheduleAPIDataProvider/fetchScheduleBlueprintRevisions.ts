import { getScheduleBlueprintsIdRevisions, type RestScheduleBlueprintRevisionResponse } from "~/api/generated/schedules"
import type { ScheduleBlueprintRevision } from "./context"

export async function fetchScheduleBlueprintRevisions(blueprintId: string, signal?: AbortSignal): Promise<ScheduleBlueprintRevision[]> {
  const revisions: RestScheduleBlueprintRevisionResponse[] = []
  let pageToken: string | undefined
  do {
    const response = await getScheduleBlueprintsIdRevisions(
      blueprintId,
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
    frequency: (revision.frequency || "once") as ScheduleBlueprintRevision["frequency"],
    interval: revision.interval || 1,
    weekdays: revision.weekdays || [],
    endDate: revision.end_date || "",
  }))
}
