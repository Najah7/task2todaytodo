import { createContext, useContext } from "react"
import type { ProjectListActions } from "~/features/project/types"

export const ProjectListActionsContext = createContext<ProjectListActions | null>(null)

export function useProjectListActions() {
  const value = useContext(ProjectListActionsContext)
  if (!value) throw new Error("ProjectListProvider is missing for project list actions")
  return value
}
