import type { RestAllocationRulesResponse, RestAllocationPlansResponse } from "~/api/generated/allocations"
import type { AllocationPlanValues } from "~/features/allocation/components/AllocationPlanForm/schema"

/** Merge independently stored plan targets and allocation rules into editor rows. */
export function allocationResources2formValues(
  rulesList: RestAllocationRulesResponse[],
  plansList: RestAllocationPlansResponse[],
): AllocationPlanValues[] {
  return rulesList.map((rules, index) => {
    const plans = plansList[index]
    const planByProject = new Map((plans?.items ?? []).map((plan) => [plan.project_id || "", plan]))
    const rulesByProject = new Map<string, NonNullable<RestAllocationRulesResponse["items"]>>()
    for (const rule of rules.items ?? []) {
      const projectId = rule.project_id || ""
      const projectRules = rulesByProject.get(projectId) ?? []
      projectRules.push(rule)
      rulesByProject.set(projectId, projectRules)
    }

    const projectIds = new Set([...planByProject.keys(), ...rulesByProject.keys()])
    return {
      projects: [...projectIds].filter(Boolean).map((projectId) => {
        const plan = planByProject.get(projectId)
        return {
          projectId,
          targetMinutes: plan?.target_minutes ?? 0,
          hasTarget: plan !== undefined,
          rules: (rulesByProject.get(projectId) ?? []).map((rule) => ({
            ...(rule.id ? { id: rule.id } : {}),
            ...(rule.revision !== undefined ? { revision: rule.revision } : {}),
            weekday: String(rule.weekday ?? 1) as AllocationPlanValues["projects"][number]["rules"][number]["weekday"],
            startTime: rule.start_time || "09:00",
            endTime: rule.end_time || "10:00",
            endDayOffset: String(rule.end_day_offset ?? 0) as "0" | "1",
            timezone: rule.timezone || rules.timezone || "UTC",
            pausedReason: rule.paused_reason || "",
          })),
        }
      }),
    }
  })
}

export function allocationFormValues2ruleRequests(valuesList: AllocationPlanValues[], timezone: string) {
  return valuesList.map((values) => values.projects.flatMap((project) => project.rules.map((rule) => ({
    ...(rule.id ? { id: rule.id } : {}),
    ...(rule.revision !== undefined ? { revision: rule.revision } : {}),
    project_id: project.projectId,
    weekday: Number(rule.weekday),
    start_time: rule.startTime,
    end_time: rule.endTime,
    end_day_offset: Number(rule.endDayOffset),
    timezone: rule.timezone || timezone,
  }))))
}

export function allocationFormValues2planRequests(valuesList: AllocationPlanValues[]) {
  return valuesList.map((values) => ({
    items: values.projects.filter((project) => project.hasTarget).map((project) => ({
      project_id: project.projectId,
      target_minutes: project.targetMinutes,
    })),
  }))
}

/** Exclude opaque server identity when comparing a draft with its saved rules. */
export function allocationFormValues2ruleConfig(valuesList: AllocationPlanValues[], timezone: string) {
  return allocationFormValues2ruleRequests(valuesList, timezone).map((rules) => rules.map(({ id: _id, revision: _revision, ...rule }) => rule))
}

export function allocationFormValues2planConfig(valuesList: AllocationPlanValues[]) {
  return allocationFormValues2planRequests(valuesList)
}

export function allocationRulesResponse2identities(responses: RestAllocationRulesResponse[], valuesList: AllocationPlanValues[]): AllocationPlanValues[] {
  return valuesList.map((values, responseIndex) => {
    const byId = new Map((responses[responseIndex]?.items ?? []).filter((rule) => rule.id).map((rule) => [rule.id!, rule]))
    const byConfig = new Map<string, NonNullable<RestAllocationRulesResponse["items"]>>()
    const byProjectWeekday = new Map<string, NonNullable<RestAllocationRulesResponse["items"]>>()
    for (const rule of responses[responseIndex]?.items ?? []) {
      const key = JSON.stringify([rule.project_id, rule.weekday, rule.start_time, rule.end_time, rule.end_day_offset, rule.timezone])
      const matches = byConfig.get(key) ?? []
      matches.push(rule)
      byConfig.set(key, matches)
      const projectWeekday = JSON.stringify([rule.project_id, rule.weekday])
      const weekdayMatches = byProjectWeekday.get(projectWeekday) ?? []
      weekdayMatches.push(rule)
      byProjectWeekday.set(projectWeekday, weekdayMatches)
    }
    return {
      projects: values.projects.map((project) => ({
        ...project,
        rules: project.rules.map((rule) => {
          const key = JSON.stringify([project.projectId, Number(rule.weekday), rule.startTime, rule.endTime, Number(rule.endDayOffset), rule.timezone])
          const projectWeekday = JSON.stringify([project.projectId, Number(rule.weekday)])
          const match = (rule.id ? byId.get(rule.id) : undefined) ?? byConfig.get(key)?.shift() ?? byProjectWeekday.get(projectWeekday)?.shift()
          return match ? { ...rule, ...(match.id ? { id: match.id } : {}), ...(match.revision !== undefined ? { revision: match.revision } : {}) } : rule
        }),
      })),
    }
  })
}
