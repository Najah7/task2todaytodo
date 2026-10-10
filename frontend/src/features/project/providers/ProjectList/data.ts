import { createContext, useContext } from "react"
import type { ProjectListData } from "~/features/project/types"

export const ProjectListDataContext = createContext<ProjectListData | null>(null)

export function useProjectListData() {
  const value = useContext(ProjectListDataContext)
  if (!value) throw new Error("ProjectListProvider is missing for project list data")
  return value
}
