import { createContext, useContext } from "react"
import type { ProjectListNavigation } from "~/features/project/types"

export const ProjectListNavigationContext = createContext<ProjectListNavigation | null>(null)

export function useProjectListNavigation() {
  const value = useContext(ProjectListNavigationContext)
  if (!value) throw new Error("ProjectListProvider is missing for project list navigation")
  return value
}
