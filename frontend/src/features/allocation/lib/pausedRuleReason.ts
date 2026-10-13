import type { MessageKey } from "~/features/i18n/hooks"

const messageKeys: Record<string, MessageKey> = {
  project_completed: "weeklyPlan.pausedRule.reason.project_completed",
  allocation_conflict: "weeklyPlan.pausedRule.reason.allocation_conflict",
  outside_active_time: "weeklyPlan.pausedRule.reason.outside_active_time",
}

export function pausedRuleReasonMessageKey(reason: string): MessageKey {
  return messageKeys[reason] ?? "weeklyPlan.pausedRule.reason.unknown"
}
