import type { MessageKey } from "~/features/i18n/types"

export type TaskStatusTabView = { value: string; label: MessageKey; count: number; selected: boolean }
export type TaskSelectOption = { value: string; label: string }
export type TaskFilterView = {
  projectId: string
  dueFilter: string
  title: string
}
export type TaskListFiltersView = {
  projectOptions: TaskSelectOption[]
  dueOptions: TaskSelectOption[]
  values: TaskFilterView
}
export type TaskActionItemRowView = {
  occurrenceKey: string
  taskId: string
  actionItemId: string
  seriesId?: string
  occurrenceDate?: string
  title: string
  estimateLabel: string
  completed: boolean
  occurrenceDateLabel: string
  todayRegistration: { timeLabel: string } | null
}
export type TaskListRowView = {
  id: string
  title: string
  canUpdate: boolean
  projectLabel: string
  statusLabel: string
  statusTone: "open" | "active" | "waiting" | "done"
  actionItemProgressLabel: string
  actionItemCount: number
  actionItemCompletedCount: number
  progress: number
  estimateLabel: string
  dueDateLabel: string
  dueDateTone: "normal" | "today" | "overdue" | "none"
  actionItems: TaskActionItemRowView[]
}
export type TaskActionItemsQueryState = "loading" | "error" | "success"
export type TaskListSummaryView = {
  totalLabel: string
  completedActionItemsLabel: string
  totalActionItemsLabel: string
  completedActionItems: number
  totalActionItems: number
  estimateTotalLabel: string
}

export type TaskListData = {
  tabs: TaskStatusTabView[]
  filters: TaskListFiltersView
  sort: string
  sortOptions: TaskSelectOption[]
  rows: TaskListRowView[]
  summary: TaskListSummaryView
  expandedIds: string[]
  busyActionItemKey?: string
  actionItemsQueryStates: Record<string, TaskActionItemsQueryState | undefined>
  loading: boolean
  error: boolean
  canGoFirst: boolean
  previousDisabled: boolean
  nextDisabled: boolean
  optionsLoading: boolean
  optionsError: boolean
}

export type TaskListNavigation = {
  selectStatus: (value: string) => void
  changeFilter: (filters: TaskFilterView) => void
  changeSort: (sort: string) => void
  first: () => void
  previous: () => void
  next: () => void
}

export type TaskListActions = {
  create: () => void
  edit: (taskId: string) => void
  toggleExpanded: (taskId: string) => void
  toggleActionItem: (item: TaskActionItemRowView) => void
  retryActionItems: (taskId: string) => void
  retry: () => void
  retryOptions: () => void
}
