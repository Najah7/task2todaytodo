import { createContext } from "react"
import type { UseQueryResult } from "@tanstack/react-query"
import type { RestProjectOptionsResponse, RestProjectResponse } from "~/api/generated/projects"
import type { RestProjectListResponse } from "~/api/generated/projects"

export type ProjectFormLoadState = "loading" | "error" | "forbidden" | "ready"

export type ProjectAPIData = {
  state: ProjectFormLoadState
  list: boolean
  projectId?: string
  project?: RestProjectResponse
  options?: RestProjectOptionsResponse
  projectsQuery: UseQueryResult<RestProjectListResponse>
  optionsQuery: UseQueryResult<RestProjectOptionsResponse>
}

export const ProjectAPIDataContext = createContext<ProjectAPIData | null>(null)
