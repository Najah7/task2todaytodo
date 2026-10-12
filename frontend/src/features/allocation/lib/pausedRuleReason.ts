import type { MessageKey } from "~/features/i18n/hooks"

const messageKeys: Record<string, MessageKey> = {
  project_completed: "weeklyPlan.pausedRule.reason.project_completed",
  allocation_conflict: "weeklyPlan.pausedRule.reason.allocation_conflict",
  sleep_conflict: "weeklyPlan.pausedRule.reason.sleep_conflict",
}

export function pausedRuleReasonMessageKey(reason: string): MessageKey {
  return messageKeys[reason] ?? "weeklyPlan.pausedRule.reason.unknown"
}
