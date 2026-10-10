import { GetProjectsSortOrder, GetProjectsStatus, GetProjectsView } from "~/api/generated/projects"
import type { ProjectListState, ProjectSortColumn, ProjectTab } from "~/features/project/providers/ProjectNavigationProvider/context"

export function projectSearchParams2projectListStates(searches: URLSearchParams[]): ProjectListState[] {
  return searches.map((search) => {
    const status = search.get("status")
    const view = search.get("view")
    const sortColumn = search.get("sort_by")
    const tab: ProjectTab = view === GetProjectsView.trash
      ? "trash"
      : status && Object.values(GetProjectsStatus).includes(status as (typeof GetProjectsStatus)[keyof typeof GetProjectsStatus])
        ? status as ProjectTab
        : GetProjectsStatus.in_progress
    const validSortColumn: ProjectSortColumn = sortColumn === "title" || sortColumn === "progress" || sortColumn === "end_date" || sortColumn === "remaining_days"
      ? sortColumn
      : "end_date"

    return {
      tab,
      sortColumn: validSortColumn,
      sortOrder: search.get("sort_order") === GetProjectsSortOrder.desc ? GetProjectsSortOrder.desc : GetProjectsSortOrder.asc,
      pageToken: search.get("page_token") || undefined,
    }
  })
}
