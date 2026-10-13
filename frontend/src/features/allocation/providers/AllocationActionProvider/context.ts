import { createContext } from "react"
import type {
  RestCreateAllocationRequest,
  RestDailyActiveTimeExceptionRequest,
  RestUpdateAllocationRequest,
  RestAllocationPlansRequest,
  RestAllocationPlansResponse,
  RestAllocationRulesRequest,
  RestAllocationRulesResponse,
  RestAllocationWeekResponse,
} from "~/api/generated/allocations"
import type { RestUserActiveTimeSettingsUpdateRequest } from "~/api/generated/auth"

export type AllocationActions = {
  busy: boolean
  create: (request: RestCreateAllocationRequest) => Promise<void>
  update: (ruleId: string, occurrenceDate: string, revision: number, request: RestUpdateAllocationRequest) => Promise<void>
  remove: (ruleId: string, occurrenceDate: string, revision: number, ruleRevision: number, scope: "current" | "following") => Promise<void>
  savePlanParts: (request: {
    revisions: { rules: string; plans: string }
    rules?: RestAllocationRulesRequest
    plans?: RestAllocationPlansRequest
  }) => Promise<{
    rules?: RestAllocationRulesResponse
    plans?: RestAllocationPlansResponse
    errors: { rules?: unknown; plans?: unknown }
  }>
  setActiveTime: (date: string, revision: number, request: RestDailyActiveTimeExceptionRequest) => Promise<void>
  resetActiveTime: (date: string, revision: number) => Promise<void>
  saveProfileActiveTime: (request: RestUserActiveTimeSettingsUpdateRequest) => Promise<void>
  refetch: () => Promise<void>
  week?: RestAllocationWeekResponse
}

export const AllocationActionContext = createContext<AllocationActions | null>(null)
