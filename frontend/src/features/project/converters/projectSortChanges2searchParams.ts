import type { GetProjectsSortOrder } from "~/api/generated/projects"
import type { ProjectSortColumn } from "~/features/project/providers/ProjectNavigationProvider/context"

type ProjectSortChange = {
  searchParams: URLSearchParams
  sortColumn: ProjectSortColumn
  sortOrder: GetProjectsSortOrder
}

export function projectSortChanges2searchParams(changes: ProjectSortChange[]): URLSearchParams[] {
  return changes.map(({ searchParams, sortColumn, sortOrder }) => {
    const next = new URLSearchParams(searchParams)
    next.delete("page_token")
    next.set("sort_by", sortColumn)
    next.set("sort_order", sortOrder)
    return next
  })
}
