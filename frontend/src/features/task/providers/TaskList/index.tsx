import type { ReactNode } from "react"
import type { TaskListActions, TaskListData, TaskListNavigation } from "~/features/task/types"
import { TaskListActionsContext } from "./action"
import { TaskListDataContext } from "./data"
import { TaskListNavigationContext } from "./navigation"

type Props = {
  data: TaskListData
  navigation: TaskListNavigation
  actions: TaskListActions
  children: ReactNode
}

export function TaskListProvider({ data, navigation, actions, children }: Props) {
  return (
    <TaskListDataContext.Provider value={data}>
      <TaskListNavigationContext.Provider value={navigation}>
        <TaskListActionsContext.Provider value={actions}>{children}</TaskListActionsContext.Provider>
      </TaskListNavigationContext.Provider>
    </TaskListDataContext.Provider>
  )
}
