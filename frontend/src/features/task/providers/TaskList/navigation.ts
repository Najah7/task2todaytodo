import { createContext, useContext } from "react"
import type { TaskListNavigation } from "~/features/task/types"

export const TaskListNavigationContext = createContext<TaskListNavigation | null>(null)

export function useTaskListNavigation() {
  const value = useContext(TaskListNavigationContext)
  if (!value) throw new Error("TaskListProvider is missing for task list navigation")
  return value
}
