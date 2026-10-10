import { GetTasksDueFilter, GetTasksSortBy, GetTasksSortOrder, GetTasksStatus } from "~/api/generated/tasks"
import type { GetTasksParams, GetTasksStatus as TaskStatus } from "~/api/generated/tasks"

export type TaskListState = {
  status: TaskStatus
  projectId?: string
  dueFilter: GetTasksParams["due_filter"]
  title: string
  sortBy: GetTasksParams["sort_by"]
  sortOrder: GetTasksSortOrder
  pageToken?: string
}

const statuses = Object.values(GetTasksStatus)
const dueFilters = Object.values(GetTasksDueFilter)
const sortFields = Object.values(GetTasksSortBy)
const sortOrders = Object.values(GetTasksSortOrder)

export function taskSearchParams2states(searches: URLSearchParams[]): TaskListState[] {
  return searches.map((search) => {
    const status = search.get("status")
    const dueFilter = search.get("due_filter")
    const sortBy = search.get("sort_by")
    const sortOrder = search.get("sort_order")
    return {
      status: statuses.includes(status as TaskStatus) ? status as TaskStatus : GetTasksStatus.all,
      projectId: search.get("project_id") || undefined,
      dueFilter: dueFilters.includes(dueFilter as NonNullable<TaskListState["dueFilter"]>) ? dueFilter as TaskListState["dueFilter"] : GetTasksDueFilter.all,
      title: search.get("title") ?? "",
      sortBy: normalizeSortBy(sortBy),
      sortOrder: normalizeSortOrder(sortBy, sortOrder),
      pageToken: search.get("page_token") || undefined,
    }
  })
}

function normalizeSortBy(sortBy: string | null): TaskListState["sortBy"] {
  return sortFields.includes(sortBy as NonNullable<TaskListState["sortBy"]>) ? sortBy as TaskListState["sortBy"] : GetTasksSortBy.due_date
}

function normalizeSortOrder(sortBy: string | null, sortOrder: string | null): GetTasksSortOrder {
  if (sortBy === GetTasksSortBy.created_at) return GetTasksSortOrder.desc
  if (sortBy === GetTasksSortBy.title) return GetTasksSortOrder.asc
  return sortOrders.includes(sortOrder as GetTasksSortOrder) ? sortOrder as GetTasksSortOrder : GetTasksSortOrder.asc
}

export function taskListStates2apiParams(states: TaskListState[]): GetTasksParams[] {
  return states.map((state) => ({
    status: state.status,
    ...(state.projectId ? { project_id: state.projectId } : {}),
    due_filter: state.dueFilter,
    ...(state.title ? { title: state.title } : {}),
    sort_by: state.sortBy,
    sort_order: state.sortOrder,
    page_size: 50,
    ...(state.pageToken ? { page_token: state.pageToken } : {}),
  }))
}

export function taskStatusChanges2searchParams(changes: { searchParams: URLSearchParams; status: TaskStatus }[]): URLSearchParams[] {
  return changes.map(({ searchParams, status }) => {
    const next = new URLSearchParams(searchParams)
    next.delete("page_token")
    next.set("status", status)
    return next
  })
}

export function taskFiltersChanges2searchParams(changes: { searchParams: URLSearchParams; projectId?: string; dueFilter?: TaskListState["dueFilter"]; title?: string }[]): URLSearchParams[] {
  return changes.map(({ searchParams, projectId, dueFilter, title }) => {
    const next = new URLSearchParams(searchParams)
    next.delete("page_token")
    setOrDelete(next, "project_id", projectId)
    setOrDelete(next, "due_filter", dueFilter && dueFilter !== GetTasksDueFilter.all ? dueFilter : undefined)
    setOrDelete(next, "title", title)
    return next
  })
}

export function taskSortChanges2searchParams(changes: { searchParams: URLSearchParams; sortBy: NonNullable<TaskListState["sortBy"]>; sortOrder: GetTasksSortOrder }[]): URLSearchParams[] {
  return changes.map(({ searchParams, sortBy, sortOrder }) => {
    const next = new URLSearchParams(searchParams)
    next.delete("page_token")
    next.set("sort_by", sortBy)
    next.set("sort_order", sortOrder)
    return next
  })
}

export function taskPageTokenChanges2searchParams(changes: { searchParams: URLSearchParams; pageToken?: string }[]): URLSearchParams[] {
  return changes.map(({ searchParams, pageToken }) => {
    const next = new URLSearchParams(searchParams)
    setOrDelete(next, "page_token", pageToken)
    return next
  })
}

function setOrDelete(search: URLSearchParams, key: string, value?: string) {
  if (value) search.set(key, value)
  else search.delete(key)
}
