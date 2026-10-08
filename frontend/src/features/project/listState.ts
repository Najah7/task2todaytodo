import {
  GetProjectsSortBy,
  GetProjectsSortOrder,
  GetProjectsStatus,
  GetProjectsView,
  type GetProjectsParams,
  type GetProjectsStatus as ProjectStatus,
} from "~/api/generated/projects"
import type { MessageKey } from "~/features/i18n/hooks"

export type ProjectTab = ProjectStatus | "trash"
export type ProjectSortColumn = "title" | "progress" | "end_date" | "remaining_days"

export const projectTabs: ProjectTab[] = [
  GetProjectsStatus.in_progress,
  GetProjectsStatus.pending,
  GetProjectsStatus.done,
  GetProjectsStatus.open,
  GetProjectsStatus.waiting_on_others,
  "trash",
]

export const projectTabLabels: Record<ProjectTab, MessageKey> = {
  [GetProjectsStatus.in_progress]: "projects.status.inProgress",
  [GetProjectsStatus.pending]: "projects.status.pending",
  [GetProjectsStatus.done]: "projects.status.done",
  [GetProjectsStatus.open]: "projects.status.open",
  [GetProjectsStatus.waiting_on_others]: "projects.status.waitingOnOthers",
  trash: "projects.status.trash",
}

export type ProjectListState = {
  tab: ProjectTab
  sortColumn: ProjectSortColumn
  sortOrder: GetProjectsSortOrder
  pageToken?: string
}

const sortColumns = new Set<ProjectSortColumn>(["title", "progress", "end_date", "remaining_days"])

function isProjectStatus(value: string | null): value is ProjectStatus {
  return value !== null && Object.values(GetProjectsStatus).includes(value as ProjectStatus)
}

function isProjectSortOrder(value: string | null): value is GetProjectsSortOrder {
  return value === GetProjectsSortOrder.asc || value === GetProjectsSortOrder.desc
}

export function readProjectListState(search: URLSearchParams): ProjectListState {
  let tab: ProjectTab = GetProjectsStatus.in_progress
  if (search.get("view") === GetProjectsView.trash) tab = "trash"
  else if (isProjectStatus(search.get("status"))) tab = search.get("status") as ProjectStatus

  const requestedSortBy = search.get("sort_by")
  const sortColumn = sortColumns.has(requestedSortBy as ProjectSortColumn)
    ? requestedSortBy as ProjectSortColumn
    : "end_date"

  return {
    tab,
    sortColumn,
    sortOrder: isProjectSortOrder(search.get("sort_order")) ? search.get("sort_order") as GetProjectsSortOrder : GetProjectsSortOrder.asc,
    pageToken: search.get("page_token") || undefined,
  }
}

export function getProjectListParams(state: ProjectListState): GetProjectsParams {
  return {
    ...(state.tab === "trash" ? { view: GetProjectsView.trash } : { status: state.tab }),
    sort_by: state.sortColumn === "remaining_days" ? GetProjectsSortBy.end_date : state.sortColumn,
    sort_order: state.sortOrder,
    page_size: 20,
    ...(state.pageToken ? { page_token: state.pageToken } : {}),
  }
}

export function setProjectTab(search: URLSearchParams, tab: ProjectTab) {
  const next = new URLSearchParams(search)
  next.delete("page_token")
  if (tab === "trash") {
    next.delete("status")
    next.set("view", GetProjectsView.trash)
  } else {
    next.delete("view")
    next.set("status", tab)
  }
  return next
}

export function setProjectSort(search: URLSearchParams, column: ProjectSortColumn, order: GetProjectsSortOrder) {
  const next = new URLSearchParams(search)
  next.delete("page_token")
  next.set("sort_by", column)
  next.set("sort_order", order)
  return next
}

export function setProjectPage(search: URLSearchParams, pageToken?: string) {
  const next = new URLSearchParams(search)
  if (pageToken) next.set("page_token", pageToken)
  else next.delete("page_token")
  return next
}
