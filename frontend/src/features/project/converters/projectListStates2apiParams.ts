import {
  GetProjectsSortBy,
  GetProjectsView,
  type GetProjectsParams,
} from "~/api/generated/projects"
import type { ProjectListState } from "~/features/project/types"

export function projectListStates2apiParams(states: ProjectListState[]): GetProjectsParams[] {
  return states.map((state) => ({
    ...(state.tab === "trash" ? { view: GetProjectsView.trash } : { status: state.tab }),
    sort_by: state.sortColumn === "remaining_days" ? GetProjectsSortBy.end_date : state.sortColumn,
    sort_order: state.sortOrder,
    page_size: 20,
    ...(state.pageToken ? { page_token: state.pageToken } : {}),
  }))
}
