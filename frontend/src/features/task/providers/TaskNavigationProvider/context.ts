import { createContext } from "react"
import type { TaskListState } from "~/features/task/converters/taskSearchParams"

export type TaskFilterView = {
  projectId: string
  dueFilter: string
  title: string
}

export type TaskNavigation = {
  state: TaskListState
  create: () => void
  edit: (taskId: string) => void
  cancel: () => void
  afterSave: () => void
  selectStatus: (value: string) => void
  changeFilter: (filters: TaskFilterView) => void
  changeSort: (sort: string) => void
  first: (options?: { replace?: boolean }) => void
  previous: (pageToken?: string) => void
  next: (pageToken?: string) => void
}

export const TaskNavigationContext = createContext<TaskNavigation | null>(null)
