import { createContext, useContext } from "react"
import type { TaskListData } from "~/features/task/types"

export const TaskListDataContext = createContext<TaskListData | null>(null)

export function useTaskListData() {
  const value = useContext(TaskListDataContext)
  if (!value) throw new Error("TaskListProvider is missing for task list data")
  return value
}
