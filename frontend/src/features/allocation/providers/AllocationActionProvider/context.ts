import { createContext } from "react"
import type {
  RestCreateAllocationRequest,
  RestDailySleepExceptionRequest,
  RestUpdateAllocationRequest,
  RestAllocationPlansRequest,
  RestAllocationPlansResponse,
  RestAllocationRulesRequest,
  RestAllocationRulesResponse,
  RestAllocationWeekResponse,
} from "~/api/generated/allocations"
import type { RestUserSleepSettingsUpdateRequest } from "~/api/generated/auth"

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
  setSleep: (date: string, revision: number, request: RestDailySleepExceptionRequest) => Promise<void>
  resetSleep: (date: string, revision: number) => Promise<void>
  saveProfileSleep: (request: RestUserSleepSettingsUpdateRequest) => Promise<void>
  refetch: () => Promise<void>
  week?: RestAllocationWeekResponse
}

export const AllocationActionContext = createContext<AllocationActions | null>(null)
