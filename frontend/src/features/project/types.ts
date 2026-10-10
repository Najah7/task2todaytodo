import type { GetProjectsSortOrder, GetProjectsStatus as ProjectStatus } from "~/api/generated/projects"
import type {
  RestProjectListSummaryResponse,
  RestProjectResponse,
  RestProjectTaskStatusResponse,
} from "~/api/generated/projects"
import type { MessageKey } from "~/features/i18n/types"
import type { ProjectFormValues } from "~/features/project/components/ProjectForm/schema"

export type ProjectTab = ProjectStatus | "trash"
export type ProjectSortColumn = "title" | "progress" | "end_date" | "remaining_days"

export type ProjectListState = {
  tab: ProjectTab
  sortColumn: ProjectSortColumn
  sortOrder: GetProjectsSortOrder
  pageToken?: string
}

export type ProjectFormOption = { value: string; label: string }

export type ProjectSubmitResult =
  | { saved: true; revision: number }
  | { saved: false; fieldErrors?: Partial<Record<keyof ProjectFormValues, MessageKey>> }

export type ProjectFormReloadSnapshot = { values: ProjectFormValues; revision: number }

export type ProjectListActions = {
  busy: boolean
  changeStatus: (project: RestProjectResponse, value: string) => void
  edit: (project: RestProjectResponse) => void
  moveToTrash: (project: RestProjectResponse) => void
  restore: (project: RestProjectResponse) => void
  rowClick: (project: RestProjectResponse) => void
}

export type ProjectListData = {
  projects: RestProjectResponse[]
  summary?: RestProjectListSummaryResponse
  statusOptions: RestProjectTaskStatusResponse[]
}

export type ProjectListNavigation = {
  state: ProjectListState
  showFirstPage: boolean
  previousDisabled: boolean
  nextDisabled: boolean
  selectTab: (tab: ProjectTab) => void
  changeSort: (column: ProjectSortColumn, order: GetProjectsSortOrder) => void
  onFirstPage: () => void
  onPrevious: () => void
  onNext: () => void
}
