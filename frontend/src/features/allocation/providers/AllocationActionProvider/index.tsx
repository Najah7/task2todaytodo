import { useContext, useRef, useState, type ReactNode } from "react"
import { useQueryClient } from "@tanstack/react-query"
import {
  deleteAllocationsRuleIdOccurrenceDate,
  deleteDailySleepExceptionsLocalDate,
  getGetAllocationPlansQueryKey,
  getGetAllocationRulesQueryKey,
  patchAllocationsRuleIdOccurrenceDate,
  postAllocations,
  putAllocationPlans,
  putAllocationRules,
  putDailySleepExceptionsLocalDate,
} from "~/api/generated/allocations"
import { getGetUsersMeQueryKey, patchUsersMeSleep } from "~/api/generated/auth"
import { useI18n } from "~/features/i18n/hooks"
import { useLanguage } from "~/features/i18n/hooks"
import type { MessageKey } from "~/features/i18n/hooks"
import { notify } from "~/features/shared/notification"
import { ApiError } from "~/api/http"
import { allocationErrorMessage } from "~/features/allocation/lib/errorMessage"
import { AllocationAPIDataContext } from "~/features/allocation/providers/AllocationAPIDataProvider/context"
import { AllocationActionContext } from "./context"

function revisionHeader(revision: number | string) {
  return { "If-Match": '"' + revision + '"' }
}

export function AllocationActionProvider({ children }: { children: ReactNode }) {
  const data = useContext(AllocationAPIDataContext)!
  const queryClient = useQueryClient()
  const i18n = useI18n()
  const { language } = useLanguage()
  const [busyCount, setBusyCount] = useState(0)
  const resourceRevisions = useRef<{ rules?: string; plans?: string }>({})
  const busy = busyCount > 0

  async function invalidateAllocationData() {
    await Promise.all([
      queryClient.invalidateQueries({
        predicate: (query) => typeof query.queryKey[0] === "string" && query.queryKey[0].startsWith("/api/allocation-weeks/"),
      }),
      queryClient.invalidateQueries({ queryKey: getGetAllocationPlansQueryKey() }),
      queryClient.invalidateQueries({ queryKey: getGetAllocationRulesQueryKey() }),
    ])
  }

  async function perform(action: () => Promise<unknown>, successKey: MessageKey) {
    setBusyCount((count) => count + 1)
    try {
      await action()
      await Promise.all([
        invalidateAllocationData(),
        queryClient.invalidateQueries({ queryKey: getGetUsersMeQueryKey() }),
      ])
      notify.success(i18n(successKey))
    } catch (error) {
      const projectNames = new Map<string, string>()
      for (const project of data.week?.projects ?? []) if (project.project_id && project.title) projectNames.set(project.project_id, project.title)
      for (const project of data.plans?.items ?? []) if (project.project_id && project.title) projectNames.set(project.project_id, project.title)
      for (const rule of data.rules?.items ?? []) if (rule.project_id && rule.project_title) projectNames.set(rule.project_id, rule.project_title)
      for (const project of data.projects) if (project.id && project.title) projectNames.set(project.id, project.title)
      const message = allocationErrorMessage(error, i18n, {
        status: error instanceof ApiError ? error.status : undefined,
        timezone: data.timezone,
        language,
        projectNames,
      })
      notify.error(message)
      throw error
    } finally {
      setBusyCount((count) => Math.max(0, count - 1))
    }
  }

  async function savePlanParts(request: { revisions: { rules: string; plans: string }; rules?: Parameters<typeof putAllocationRules>[0]; plans?: Parameters<typeof putAllocationPlans>[0] }) {
    const revisions = {
      rules: resourceRevisions.current.rules ?? request.revisions.rules,
      plans: resourceRevisions.current.plans ?? request.revisions.plans,
    }
    setBusyCount((count) => count + 1)
    try {
      const [rulesResult, plansResult] = await Promise.all([
        request.rules
          ? putAllocationRules(request.rules, { headers: revisionHeader(revisions.rules) }).then((response) => ({ status: "fulfilled" as const, value: response }), (reason: unknown) => ({ status: "rejected" as const, reason }))
          : Promise.resolve(undefined),
        request.plans
          ? putAllocationPlans(request.plans, { headers: revisionHeader(revisions.plans) }).then((response) => ({ status: "fulfilled" as const, value: response }), (reason: unknown) => ({ status: "rejected" as const, reason }))
          : Promise.resolve(undefined),
      ])

      const errors: { rules?: unknown; plans?: unknown } = {}
      const result: { rules?: Awaited<ReturnType<typeof putAllocationRules>>; plans?: Awaited<ReturnType<typeof putAllocationPlans>>; errors: typeof errors } = { errors }
      if (rulesResult?.status === "fulfilled") {
        result.rules = rulesResult.value
        resourceRevisions.current.rules = rulesResult.value.revision!
        queryClient.setQueryData(getGetAllocationRulesQueryKey(), rulesResult.value)
      } else if (rulesResult?.status === "rejected") errors.rules = rulesResult.reason
      if (plansResult?.status === "fulfilled") {
        result.plans = plansResult.value
        resourceRevisions.current.plans = plansResult.value.revision!
        queryClient.setQueryData(getGetAllocationPlansQueryKey(), plansResult.value)
      } else if (plansResult?.status === "rejected") errors.plans = plansResult.reason

      if (result.rules || result.plans) await invalidateAllocationData()
      if ((request.rules === undefined || result.rules) && (request.plans === undefined || result.plans)) {
        notify.success(i18n("weeklyPlan.toast.saved"))
      }
      return result
    } finally {
      setBusyCount((count) => Math.max(0, count - 1))
    }
  }

  const value = {
    busy,
    week: data.week,
    refetch: data.refetch,
    create: (request: Parameters<typeof postAllocations>[0]) =>
      perform(() => postAllocations(request), "weeklyPlan.toast.created"),
    update: (ruleId: string, occurrenceDate: string, revision: number, request: Parameters<typeof patchAllocationsRuleIdOccurrenceDate>[2]) =>
      perform(() => patchAllocationsRuleIdOccurrenceDate(ruleId, occurrenceDate, request, { headers: revisionHeader(revision) }), "weeklyPlan.toast.saved"),
    remove: (ruleId: string, occurrenceDate: string, revision: number, ruleRevision: number, scope: "current" | "following") =>
      perform(() => deleteAllocationsRuleIdOccurrenceDate(ruleId, occurrenceDate, { scope, rule_revision: ruleRevision }, { headers: revisionHeader(revision) }), "weeklyPlan.toast.deleted"),
    savePlanParts,
    setSleep: (date: string, revision: number, request: Parameters<typeof putDailySleepExceptionsLocalDate>[1]) =>
      perform(() => putDailySleepExceptionsLocalDate(date, request, { headers: revisionHeader(revision) }), "weeklyPlan.toast.sleepSaved"),
    resetSleep: (date: string, revision: number) =>
      perform(() => deleteDailySleepExceptionsLocalDate(date, { headers: revisionHeader(revision) }), "weeklyPlan.toast.sleepReset"),
    saveProfileSleep: (request: Parameters<typeof patchUsersMeSleep>[0]) =>
      perform(() => patchUsersMeSleep(request), "weeklyPlan.toast.profileSaved"),
  }

  return <AllocationActionContext.Provider value={value}>{children}</AllocationActionContext.Provider>
}
