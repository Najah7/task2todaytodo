import { createContext } from "react"
import type { GetProjectsSortOrder, GetProjectsStatus as ProjectStatus } from "~/api/generated/projects"

export type ProjectTab = ProjectStatus | "trash"
export type ProjectSortColumn = "title" | "progress" | "end_date" | "remaining_days"

export type ProjectListState = {
  tab: ProjectTab
  sortColumn: ProjectSortColumn
  sortOrder: GetProjectsSortOrder
  pageToken?: string
}

export type ProjectNavigation = {
  state: ProjectListState
  cancel: () => void
  afterSave: (status?: string) => void
  create: () => void
  edit: (projectId: string) => void
  selectTab: (tab: ProjectTab) => void
  changeSort: (column: ProjectSortColumn, order: GetProjectsSortOrder) => void
  first: (options?: { replace?: boolean }) => void
  previous: (pageToken?: string) => void
  next: (pageToken?: string) => void
}

export const ProjectNavigationContext = createContext<ProjectNavigation | null>(null)
