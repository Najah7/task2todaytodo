import { createContext, useContext } from "react"
import type { TaskListActions } from "~/features/task/types"

export const TaskListActionsContext = createContext<TaskListActions | null>(null)

export function useTaskListActions() {
  const value = useContext(TaskListActionsContext)
  if (!value) throw new Error("TaskListProvider is missing for task list actions")
  return value
}
