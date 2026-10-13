import { createContext } from "react"
import type { UseQueryResult } from "@tanstack/react-query"
import type { RestUserResponse } from "~/api/generated/auth"
import type { RestAllocationPlansResponse, RestAllocationRulesResponse, RestAllocationWeekResponse } from "~/api/generated/allocations"
import type { RestProjectResponse } from "~/api/generated/projects"

export type AllocationAPIDataState = "loading" | "error" | "forbidden" | "ready"

export type AllocationAPIData = {
  state: AllocationAPIDataState
  isPaid: boolean
  weekStart: string
  timezone: string
  week?: RestAllocationWeekResponse
  rules?: RestAllocationRulesResponse
  plans?: RestAllocationPlansResponse
  user?: RestUserResponse
  projects: RestProjectResponse[]
  weekQuery: UseQueryResult<RestAllocationWeekResponse>
  rulesQuery: UseQueryResult<RestAllocationRulesResponse>
  plansQuery: UseQueryResult<RestAllocationPlansResponse>
  userQuery: UseQueryResult<RestUserResponse>
  projectsQuery: UseQueryResult<RestProjectResponse[]>
  refetch: () => Promise<void>
}

export const AllocationAPIDataContext = createContext<AllocationAPIData | null>(null)
