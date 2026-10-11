import { useContext, useState, type ReactNode } from "react"
import { useQueryClient } from "@tanstack/react-query"
import {
  deleteScheduleBlueprintsIdFollowingOccurrenceDate,
  deleteSchedulesBlueprintIdOccurrenceDate,
  getGetScheduleBlueprintsIdQueryKey,
  getScheduleBlueprintsId,
  getSchedulesBlueprintIdOccurrenceDate,
  patchScheduleBlueprintsId,
  patchSchedulesBlueprintIdOccurrenceDate,
  postScheduleBlueprints,
  postSchedulesBlueprintIdOccurrenceDatecomplete,
  postSchedulesBlueprintIdOccurrenceDaterestore,
  postSchedulesBlueprintIdOccurrenceDatereopen,
  postSchedulesBlueprintIdOccurrenceDateskip,
  type RestScheduleOccurrenceUpdateRequest,
} from "~/api/generated/schedules"
import { notify } from "~/features/shared/notification"
import { useI18n } from "~/features/i18n/hooks"
import { scheduleEvents2formValues } from "~/features/schedule/converters/scheduleEvents2formValues"
import { scheduleBlueprintRevisions2formValues } from "~/features/schedule/converters/scheduleBlueprintRevisions2formValues"
import { selectEffectiveBlueprintRevision } from "~/features/schedule/providers/ScheduleAPIDataProvider/selectEffectiveBlueprintRevision"
import { fetchScheduleBlueprintRevisions } from "~/features/schedule/providers/ScheduleAPIDataProvider/fetchScheduleBlueprintRevisions"
import { ScheduleAPIDataContext, type ScheduleBlueprintRevision } from "~/features/schedule/providers/ScheduleAPIDataProvider/context"
import { scheduleOccurrences2events } from "~/features/schedule/converters/scheduleOccurrences2events"
import { scheduleFormValues2blueprintCreateRequests } from "~/features/schedule/converters/scheduleFormValues2blueprintCreateRequests"
import { scheduleFormValues2blueprintUpdateRequests } from "~/features/schedule/converters/scheduleFormValues2blueprintUpdateRequests"
import { scheduleFormValues2occurrenceUpdateRequests } from "~/features/schedule/converters/scheduleFormValues2occurrenceUpdateRequests"
import { ScheduleActionContext, type ScheduleActions, type ScheduleEditInput } from "./context"

type Props = { children: ReactNode }

export function ScheduleActionProvider({ children }: Props) {
  const data = useContext(ScheduleAPIDataContext)!
  const queryClient = useQueryClient()
  const t = useI18n()
  const [busy, setBusy] = useState(false)

  async function perform(action: () => Promise<unknown>, notification: "saved" | "created" | "deleted" | "completed" | "reopened" | "skipped" | "restored") {
    setBusy(true)
    try {
      await action()
      await queryClient.invalidateQueries({ queryKey: ["schedules"] })
      notify.success(t(`schedules.notification.${notification}`))
    } catch (error) {
      await queryClient.invalidateQueries({ queryKey: ["schedules"] })
      notify.error(t("schedules.error.requestFailed"))
      throw error
    } finally {
      setBusy(false)
    }
  }

  const actions: ScheduleActions = {
    busy,
    create: (values) => perform(() => postScheduleBlueprints(scheduleFormValues2blueprintCreateRequests([values])[0]), "created"),
    update: (input) => perform(() => updateSchedule(input), "saved"),
    complete: (event) => perform(() => postSchedulesBlueprintIdOccurrenceDatecomplete(event.blueprintId, event.occurrenceDate, { headers: revisionHeaders(event.revision) }), "completed"),
    reopen: (event) => perform(() => postSchedulesBlueprintIdOccurrenceDatereopen(event.blueprintId, event.occurrenceDate, { headers: revisionHeaders(event.revision) }), "reopened"),
    skip: (event) => perform(() => postSchedulesBlueprintIdOccurrenceDateskip(event.blueprintId, event.occurrenceDate, { headers: revisionHeaders(event.revision) }), "skipped"),
    restore: (event) => perform(() => postSchedulesBlueprintIdOccurrenceDaterestore(event.blueprintId, event.occurrenceDate, { headers: revisionHeaders(event.revision) }), "restored"),
    delete: (event, scope) => perform(async () => {
      if (scope === "following") {
        const blueprint = await queryClient.fetchQuery({
          queryKey: getGetScheduleBlueprintsIdQueryKey(event.blueprintId),
          queryFn: () => getScheduleBlueprintsId(event.blueprintId),
        })
        return deleteScheduleBlueprintsIdFollowingOccurrenceDate(event.blueprintId, event.occurrenceDate, {
          headers: revisionHeaders(blueprint.revision || 0),
        })
      }
      return deleteSchedulesBlueprintIdOccurrenceDate(event.blueprintId, event.occurrenceDate, { headers: revisionHeaders(event.revision) })
    }, "deleted"),
    reloadLatest: async (event, scope, currentValues) => {
      const occurrence = await getSchedulesBlueprintIdOccurrenceDate(event.blueprintId, event.occurrenceDate)
      if (scope === "current" && occurrence.can_update !== true && occurrence.can_assign !== true) throw new Error("schedule permission changed")
      if (scope === "following" && occurrence.can_update_following !== true) throw new Error("schedule permission changed")
      const occurrenceKey = ["schedules", "occurrence", event.blueprintId, event.occurrenceDate]
      const mapped = scheduleOccurrences2events([occurrence], data.projectOptions)[0]
      if (scope === "current") {
        const [revisionResult, blueprintResult] = await Promise.allSettled([
          queryClient.fetchQuery({
            queryKey: ["schedules", "blueprint-revisions", event.blueprintId],
            queryFn: () => fetchScheduleBlueprintRevisions(event.blueprintId),
          }),
          queryClient.fetchQuery({
            queryKey: getGetScheduleBlueprintsIdQueryKey(event.blueprintId),
            queryFn: () => getScheduleBlueprintsId(event.blueprintId),
          }),
        ])
        const revisions = revisionResult.status === "fulfilled"
          ? revisionResult.value
          : queryClient.getQueryData<ScheduleBlueprintRevision[]>(["schedules", "blueprint-revisions", event.blueprintId]) || []
        const effective = selectEffectiveBlueprintRevision(revisions, event.occurrenceDate, occurrence.source_revision_id)
        const values = scheduleEvents2formValues([mapped], effective ? [effective] : [])[0]
        if (!effective && currentValues) {
          values.frequency = currentValues.frequency
          values.interval = currentValues.interval
          values.weekdays = currentValues.weekdays
          values.recurrenceEndDate = currentValues.recurrenceEndDate
          values.recurrenceAnchorDate = currentValues.recurrenceAnchorDate
        }
        const aggregateRevision = blueprintResult.status === "fulfilled" ? blueprintResult.value.revision : undefined
        queryClient.setQueryData(occurrenceKey, occurrence)
        return {
          values,
          occurrenceRevision: occurrence.revision || 0,
          blueprintRevision: aggregateRevision,
          followingAvailable: Boolean(revisionResult.status === "fulfilled" && blueprintResult.status === "fulfilled" && effective
            && Number.isInteger(aggregateRevision) && (aggregateRevision || 0) > 0 && occurrence.can_update_following),
        }
      }
      const [revisions, blueprint] = await Promise.all([
        queryClient.fetchQuery({
          queryKey: ["schedules", "blueprint-revisions", event.blueprintId],
          queryFn: () => fetchScheduleBlueprintRevisions(event.blueprintId),
        }),
        queryClient.fetchQuery({
          queryKey: getGetScheduleBlueprintsIdQueryKey(event.blueprintId),
          queryFn: () => getScheduleBlueprintsId(event.blueprintId),
        }),
      ])
      const effective = selectEffectiveBlueprintRevision(revisions, event.occurrenceDate, occurrence.source_revision_id)
      if (!effective) throw new Error("schedule definition unavailable")
      queryClient.setQueryData(occurrenceKey, occurrence)
      return {
        values: scheduleBlueprintRevisions2formValues([effective], event.occurrenceDate)[0],
        occurrenceRevision: occurrence.revision || 0,
        blueprintRevision: blueprint.revision || 0,
        followingAvailable: true,
      }
    },
  }

  return <ScheduleActionContext.Provider value={actions}>{children}</ScheduleActionContext.Provider>

  async function updateSchedule(input: ScheduleEditInput) {
    if (input.scope === "following") {
      return patchScheduleBlueprintsId(input.blueprintId, scheduleFormValues2blueprintUpdateRequests([input.values], [input.baselineValues])[0], {
        headers: revisionHeaders(input.blueprintRevision),
      })
    }
    let request: RestScheduleOccurrenceUpdateRequest
    if (data.event?.permissions.canUpdate) {
      request = scheduleFormValues2occurrenceUpdateRequests([input.values], [input.baselineValues])[0]
    } else {
      if (!input.values.assigneeId) return
      request = { assignee_id: input.values.assigneeId }
    }
    return patchSchedulesBlueprintIdOccurrenceDate(input.blueprintId, input.occurrenceDate, request, {
      headers: revisionHeaders(input.revision),
    })
  }
}

function revisionHeaders(revision: number): Record<string, string> {
  return { "If-Match": `"${revision}"` }
}
