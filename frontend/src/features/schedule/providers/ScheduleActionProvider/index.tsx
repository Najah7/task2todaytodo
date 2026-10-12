import { useContext, useRef, useState, type ReactNode } from "react"
import { useQueryClient } from "@tanstack/react-query"
import { getListInboxQueryKey } from "~/api/generated/inbox"
import { getGetTodayTodoQueryKey } from "~/api/generated/todo"
import {
  deleteScheduleRulesIdFollowingOccurrenceDate,
  deleteSchedulesRuleIdOccurrenceDate,
  getGetScheduleRulesIdQueryKey,
  getScheduleRulesId,
  getSchedulesRuleIdOccurrenceDate,
  patchScheduleRulesId,
  patchSchedulesRuleIdOccurrenceDate,
  postScheduleRules,
  postSchedulesRuleIdOccurrenceDatecomplete,
  postSchedulesRuleIdOccurrenceDaterestore,
  postSchedulesRuleIdOccurrenceDatereopen,
  postSchedulesRuleIdOccurrenceDateskip,
  type RestScheduleOccurrenceUpdateRequest,
} from "~/api/generated/schedules"
import { notify } from "~/features/shared/notification"
import { isTodoRegenerationRequired } from "~/api/http"
import ConfirmationDialog from "~/features/shared/components/ConfirmationDialog"
import { useI18n } from "~/features/i18n/hooks"
import { scheduleEvents2formValues } from "~/features/schedule/converters/scheduleEvents2formValues"
import { scheduleRuleRevisions2formValues } from "~/features/schedule/converters/scheduleRuleRevisions2formValues"
import { selectEffectiveRuleRevision } from "~/features/schedule/providers/ScheduleAPIDataProvider/selectEffectiveRuleRevision"
import { fetchScheduleRuleRevisions } from "~/features/schedule/providers/ScheduleAPIDataProvider/fetchScheduleRuleRevisions"
import { ScheduleAPIDataContext, type ScheduleRuleRevision } from "~/features/schedule/providers/ScheduleAPIDataProvider/context"
import { scheduleOccurrences2events } from "~/features/schedule/converters/scheduleOccurrences2events"
import { scheduleFormValues2ruleCreateRequests } from "~/features/schedule/converters/scheduleFormValues2ruleCreateRequests"
import { scheduleFormValues2ruleUpdateRequests } from "~/features/schedule/converters/scheduleFormValues2ruleUpdateRequests"
import { scheduleFormValues2occurrenceUpdateRequests } from "~/features/schedule/converters/scheduleFormValues2occurrenceUpdateRequests"
import { ScheduleActionContext, type ScheduleActions, type ScheduleEditInput } from "./context"
import { RegenerationConfirmationCancelledError } from "./RegenerationConfirmationCancelledError"

type Props = { children: ReactNode }

export function ScheduleActionProvider({ children }: Props) {
  const data = useContext(ScheduleAPIDataContext)!
  const queryClient = useQueryClient()
  const t = useI18n()
  const [busy, setBusy] = useState(false)
  const [regenerationConfirmationOpen, setRegenerationConfirmationOpen] = useState(false)
  const regenerationConfirmationResolver = useRef<((confirmed: boolean) => void) | undefined>(undefined)

  function requestRegenerationConfirmation() {
    setRegenerationConfirmationOpen(true)
    return new Promise<boolean>((resolve) => { regenerationConfirmationResolver.current = resolve })
  }

  function settleRegenerationConfirmation(confirmed: boolean) {
    setRegenerationConfirmationOpen(false)
    regenerationConfirmationResolver.current?.(confirmed)
    regenerationConfirmationResolver.current = undefined
  }

  async function perform(action: (confirmRegeneration: boolean) => Promise<unknown>, notification: "saved" | "created" | "deleted" | "completed" | "reopened" | "skipped" | "restored") {
    setBusy(true)
    try {
      try {
        await action(false)
      } catch (error) {
        if (!isTodoRegenerationRequired(error)) throw error
        const confirmed = await requestRegenerationConfirmation()
        if (!confirmed) throw new RegenerationConfirmationCancelledError()
        await action(true)
      }
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["schedules"] }),
        queryClient.invalidateQueries({ queryKey: getListInboxQueryKey() }),
        queryClient.invalidateQueries({ queryKey: getGetTodayTodoQueryKey() }),
      ])
      notify.success(t(`schedules.notification.${notification}`))
    } catch (error) {
      if (error instanceof RegenerationConfirmationCancelledError) throw error
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["schedules"] }),
        queryClient.invalidateQueries({ queryKey: getListInboxQueryKey() }),
        queryClient.invalidateQueries({ queryKey: getGetTodayTodoQueryKey() }),
      ])
      notify.error(t("schedules.error.requestFailed"))
      throw error
    } finally {
      setBusy(false)
    }
  }

  const actions: ScheduleActions = {
    busy,
    create: (values) => perform((confirmRegeneration) => {
      const request = scheduleFormValues2ruleCreateRequests([values])[0]
      return postScheduleRules({ ...request, ...(confirmRegeneration ? { confirm_todo_regeneration: true } : {}) })
    }, "created"),
    update: (input) => perform((confirmRegeneration) => updateSchedule(input, confirmRegeneration), "saved"),
    complete: (event) => perform(() => postSchedulesRuleIdOccurrenceDatecomplete(event.ruleId, event.occurrenceDate, { headers: revisionHeaders(event.revision) }), "completed"),
    reopen: (event) => perform(() => postSchedulesRuleIdOccurrenceDatereopen(event.ruleId, event.occurrenceDate, { headers: revisionHeaders(event.revision) }), "reopened"),
    skip: (event) => perform((confirmRegeneration) => postSchedulesRuleIdOccurrenceDateskip(event.ruleId, event.occurrenceDate, { confirm_todo_regeneration: confirmRegeneration || undefined }, { headers: revisionHeaders(event.revision) }), "skipped"),
    restore: (event) => perform((confirmRegeneration) => postSchedulesRuleIdOccurrenceDaterestore(event.ruleId, event.occurrenceDate, { confirm_todo_regeneration: confirmRegeneration || undefined }, { headers: revisionHeaders(event.revision) }), "restored"),
    delete: (event, scope) => perform(async (confirmRegeneration) => {
      if (scope === "following") {
        const rule = await queryClient.fetchQuery({
          queryKey: getGetScheduleRulesIdQueryKey(event.ruleId),
          queryFn: () => getScheduleRulesId(event.ruleId),
        })
        return deleteScheduleRulesIdFollowingOccurrenceDate(event.ruleId, event.occurrenceDate, { confirm_todo_regeneration: confirmRegeneration || undefined }, {
          headers: revisionHeaders(rule.revision || 0),
        })
      }
      return deleteSchedulesRuleIdOccurrenceDate(event.ruleId, event.occurrenceDate, { confirm_todo_regeneration: confirmRegeneration || undefined }, { headers: revisionHeaders(event.revision) })
    }, "deleted"),
    reloadLatest: async (event, scope, currentValues) => {
      const occurrence = await getSchedulesRuleIdOccurrenceDate(event.ruleId, event.occurrenceDate)
      if (scope === "current" && occurrence.can_update !== true && occurrence.can_assign !== true) throw new Error("schedule permission changed")
      if (scope === "following" && occurrence.can_update_following !== true) throw new Error("schedule permission changed")
      const occurrenceKey = ["schedules", "occurrence", event.ruleId, event.occurrenceDate]
      const mapped = scheduleOccurrences2events([occurrence], data.projectOptions)[0]
      if (scope === "current") {
        const [revisionResult, ruleResult] = await Promise.allSettled([
          queryClient.fetchQuery({
            queryKey: ["schedules", "rule-revisions", event.ruleId],
            queryFn: () => fetchScheduleRuleRevisions(event.ruleId),
          }),
          queryClient.fetchQuery({
            queryKey: getGetScheduleRulesIdQueryKey(event.ruleId),
            queryFn: () => getScheduleRulesId(event.ruleId),
          }),
        ])
        const revisions = revisionResult.status === "fulfilled"
          ? revisionResult.value
          : queryClient.getQueryData<ScheduleRuleRevision[]>(["schedules", "rule-revisions", event.ruleId]) || []
        const effective = selectEffectiveRuleRevision(revisions, event.occurrenceDate, occurrence.source_revision_id)
        const values = scheduleEvents2formValues([mapped], effective ? [effective] : [])[0]
        if (!effective && currentValues) {
          values.frequency = currentValues.frequency
          values.interval = currentValues.interval
          values.weekdays = currentValues.weekdays
          values.recurrenceEndDate = currentValues.recurrenceEndDate
          values.recurrenceAnchorDate = currentValues.recurrenceAnchorDate
        }
        const aggregateRevision = ruleResult.status === "fulfilled" ? ruleResult.value.revision : undefined
        queryClient.setQueryData(occurrenceKey, occurrence)
        return {
          values,
          occurrenceRevision: occurrence.revision || 0,
          ruleRevision: aggregateRevision,
          followingAvailable: Boolean(revisionResult.status === "fulfilled" && ruleResult.status === "fulfilled" && effective
            && Number.isInteger(aggregateRevision) && (aggregateRevision || 0) > 0 && occurrence.can_update_following),
        }
      }
      const [revisions, rule] = await Promise.all([
        queryClient.fetchQuery({
          queryKey: ["schedules", "rule-revisions", event.ruleId],
          queryFn: () => fetchScheduleRuleRevisions(event.ruleId),
        }),
        queryClient.fetchQuery({
          queryKey: getGetScheduleRulesIdQueryKey(event.ruleId),
          queryFn: () => getScheduleRulesId(event.ruleId),
        }),
      ])
      const effective = selectEffectiveRuleRevision(revisions, event.occurrenceDate, occurrence.source_revision_id)
      if (!effective) throw new Error("schedule definition unavailable")
      queryClient.setQueryData(occurrenceKey, occurrence)
      return {
        values: scheduleRuleRevisions2formValues([effective], event.occurrenceDate)[0],
        occurrenceRevision: occurrence.revision || 0,
        ruleRevision: rule.revision || 0,
        followingAvailable: true,
      }
    },
  }

  return <ScheduleActionContext.Provider value={actions}>
    {children}
    <ConfirmationDialog open={regenerationConfirmationOpen} title={t("today.confirm.sourceChangeTitle")} description={t("today.confirm.sourceChangeDescription")} confirmLabel={t("today.confirm.regenerate")} cancelLabel={t("today.confirm.cancel")} onCancel={() => settleRegenerationConfirmation(false)} onConfirm={() => settleRegenerationConfirmation(true)} />
  </ScheduleActionContext.Provider>

  async function updateSchedule(input: ScheduleEditInput, confirmRegeneration: boolean) {
    if (input.scope === "following") {
      const request = scheduleFormValues2ruleUpdateRequests([input.values], [input.baselineValues])[0]
      return patchScheduleRulesId(input.ruleId, { ...request, ...(confirmRegeneration ? { confirm_todo_regeneration: true } : {}) }, {
        headers: revisionHeaders(input.ruleRevision),
      })
    }
    let request: RestScheduleOccurrenceUpdateRequest
    if (data.event?.permissions.canUpdate) {
      request = scheduleFormValues2occurrenceUpdateRequests([input.values], [input.baselineValues])[0]
    } else {
      if (!input.values.assigneeId) return
      request = { assignee_id: input.values.assigneeId }
    }
    if (confirmRegeneration) request.confirm_todo_regeneration = true
    return patchSchedulesRuleIdOccurrenceDate(input.ruleId, input.occurrenceDate, request, {
      headers: revisionHeaders(input.revision),
    })
  }
}


function revisionHeaders(revision: number): Record<string, string> {
  return { "If-Match": `"${revision}"` }
}
