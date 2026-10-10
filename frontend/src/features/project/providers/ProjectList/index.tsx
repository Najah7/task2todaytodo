import type { ReactNode } from "react"
import { ProjectListActionsContext } from "./action"
import { ProjectListDataContext } from "./data"
import { ProjectListNavigationContext } from "./navigation"
import type { ProjectListActions, ProjectListData, ProjectListNavigation } from "~/features/project/types"

type Props = {
  data: ProjectListData
  navigation: ProjectListNavigation
  actions: ProjectListActions
  children: ReactNode
}

export function ProjectListProvider({ data, navigation, actions, children }: Props) {
  return (
    <ProjectListDataContext.Provider value={data}>
      <ProjectListNavigationContext.Provider value={navigation}>
        <ProjectListActionsContext.Provider value={actions}>{children}</ProjectListActionsContext.Provider>
      </ProjectListNavigationContext.Provider>
    </ProjectListDataContext.Provider>
  )
}
