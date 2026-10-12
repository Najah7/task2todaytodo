import { useEffect, useMemo, useRef, type ReactNode } from "react"
import { useQuery, useQueryClient, type UseQueryResult } from "@tanstack/react-query"
import { Temporal } from "@js-temporal/polyfill"
import { useSearchParams } from "react-router"
import { ApiError } from "~/api/http"
import { notify } from "~/features/shared/notification"
import { getGetUsersMeQueryKey, getUsersMe } from "~/api/generated/auth"
import { getTimezones } from "~/api/generated/auth"
import { useI18n } from "~/features/i18n/hooks"
import {
  getGetScheduleRulesIdQueryKey,
  getGetScheduleRulesIdAssigneesQueryKey,
  getProjectsIdSchedules,
  getScheduleRulesId,
  getScheduleRulesIdAssignees,
  getSchedules,
  getSchedulesRuleIdOccurrenceDate,
  type GetProjectsIdSchedulesParams,
  type GetSchedulesParams,
  type RestScheduleOccurrenceResponse,
} from "~/api/generated/schedules"
import {
  getGetProjectsIdMembersQueryKey,
  getGetProjectsIdQueryKey,
  getProjects,
  getProjectsId,
  getProjectsIdMembers,
  type GetProjectsParams,
  type RestProjectResponse,
} from "~/api/generated/projects"
import { scheduleOccurrences2events } from "~/features/schedule/converters/scheduleOccurrences2events"
import { projectMembers2assigneeOptions } from "~/features/schedule/converters/projectMembers2assigneeOptions"
import { scheduleAssignees2options } from "~/features/schedule/converters/scheduleAssignees2options"
import { scheduleRange, scheduleEventOverlapsRange } from "~/features/schedule/lib/scheduleRange"
import { scheduleSearchParams2state } from "~/features/schedule/converters/scheduleSearchParams"
import { ScheduleAPIDataContext, type ScheduleRuleRevision } from "./context"
import { selectEffectiveRuleRevision } from "./selectEffectiveRuleRevision"
import { fetchScheduleRuleRevisions } from "./fetchScheduleRuleRevisions"

type Props = { children: ReactNode; list?: boolean; ruleId?: string; occurrenceDate?: string }

export function ScheduleAPIDataProvider({ children, list = false, ruleId, occurrenceDate }: Props) {
  const [searchParams] = useSearchParams()
  const queryClient = useQueryClient()
  const t = useI18n()
  const userQuery = useQuery({ queryKey: getGetUsersMeQueryKey(), queryFn: ({ signal }) => getUsersMe({ signal }) })
  const timezoneQuery = useQuery({ queryKey: ["schedules", "timezones"], queryFn: ({ signal }) => getTimezones({ signal }) })
  const projectsQuery = useQuery({
    queryKey: ["schedules", "projects"],
    queryFn: ({ signal }) => fetchProjects(signal),
  })
  const timezone = userQuery.data?.timezone || "UTC"
  const today = Temporal.Now.plainDateISO(timezone).toString()
  const navigation = scheduleSearchParams2state(searchParams, today)
  const range = useMemo(() => scheduleRange(navigation.date, navigation.view, timezone), [navigation.date, navigation.view, timezone])
  const scheduleQuery = useQuery({
    queryKey: ["schedules", "occurrences", navigation.projectId || "personal", range.from, range.to, navigation.showSkipped],
    enabled: list,
    queryFn: ({ signal }) => fetchOccurrences(
      range,
      navigation.projectId,
      navigation.showSkipped,
      signal,
    ),
  })
  const ruleQuery = useQuery({
    queryKey: getGetScheduleRulesIdQueryKey(ruleId || ""),
    enabled: Boolean(ruleId),
    queryFn: ({ signal }) => getScheduleRulesId(ruleId!, { signal }),
  })
  const occurrenceQuery = useQuery({
    queryKey: ["schedules", "occurrence", ruleId, occurrenceDate],
    enabled: Boolean(ruleId && occurrenceDate && !list),
    queryFn: ({ signal }) => getSchedulesRuleIdOccurrenceDate(ruleId!, occurrenceDate!, { signal }),
  })
  const revisionsQuery = useQuery<ScheduleRuleRevision[]>({
    queryKey: ["schedules", "rule-revisions", ruleId],
    enabled: Boolean(ruleId),
    queryFn: ({ signal }) => fetchScheduleRuleRevisions(ruleId!, signal),
  })
  const assigneesQuery = useQuery({
    queryKey: getGetScheduleRulesIdAssigneesQueryKey(ruleId || ""),
    enabled: Boolean(ruleId && !list && (occurrenceQuery.data?.can_assign || occurrenceQuery.data?.can_update_following)),
    queryFn: ({ signal }) => getScheduleRulesIdAssignees(ruleId!, { signal }),
  })

  const rule = ruleQuery.data
  const selectedProjectId = occurrenceQuery.data?.project_id || rule?.project_id || navigation.projectId
  const membersQuery = useQuery({
    queryKey: getGetProjectsIdMembersQueryKey(selectedProjectId || ""),
    enabled: Boolean(selectedProjectId && !list),
    queryFn: ({ signal }) => getProjectsIdMembers(selectedProjectId!, undefined, { signal }),
  })
  const projectQuery = useQuery({
    queryKey: getGetProjectsIdQueryKey(selectedProjectId || ""),
    enabled: Boolean(selectedProjectId && !list),
    queryFn: ({ signal }) => getProjectsId(selectedProjectId!, { signal }),
  })

  const projectOptions = useMemo(() => (projectsQuery.data || []).map((project) => ({ id: project.id, title: project.title, ownerId: project.user_id })), [projectsQuery.data])
  const assigneeOptions = useMemo(() => assigneesQuery.data && selectedProjectId === rule?.project_id
    ? scheduleAssignees2options(assigneesQuery.data.items || [], userQuery.data, t("schedules.projectOwner"))
    : projectMembers2assigneeOptions(membersQuery.data?.items || [], projectQuery.data, userQuery.data, t("schedules.projectOwner"), projectOptions.find((item) => item.id === selectedProjectId)?.ownerId, t("schedules.currentUser")),
  [assigneesQuery.data, rule?.project_id, membersQuery.data, projectOptions, projectQuery.data, selectedProjectId, t, userQuery.data])
  const revisions = revisionsQuery.data || []
  const occurrence = occurrenceQuery.data
  const rawEvent = occurrence ? scheduleOccurrences2events([occurrence], projectOptions)[0] : undefined
  const effectiveRevision = occurrence
    ? selectEffectiveRuleRevision(revisions, occurrenceDate || occurrence.occurrence_date || "", occurrence.source_revision_id)
    : undefined
  const event = rawEvent ? {
    ...rawEvent,
    ruleRevision: rule?.revision,
    ...(effectiveRevision ? { recurrence: {
      frequency: effectiveRevision.frequency,
      interval: effectiveRevision.interval,
      weekdays: effectiveRevision.weekdays,
      endDate: effectiveRevision.endDate,
    } } : {}),
  } : undefined
  const visibleEvents = useMemo(() => {
    if (!scheduleQuery.data) return []
    return scheduleOccurrences2events(scheduleQuery.data, projectOptions)
      .filter((item) => scheduleEventOverlapsRange(item, range, timezone))
  }, [scheduleQuery.data, range, projectOptions, timezone])

  const relevantQueries: UseQueryResult<unknown>[] = [userQuery, timezoneQuery, projectsQuery]
  if (list) relevantQueries.push(scheduleQuery)
  if (ruleId && occurrenceDate && !list) relevantQueries.push(occurrenceQuery)
  const optionalQueries = [
    ...(ruleId ? [ruleQuery, revisionsQuery] : []),
    ...(selectedProjectId && !list ? [membersQuery, projectQuery] : []),
    ...(ruleId && !list ? [assigneesQuery] : []),
  ]
  const reportableQueries = [...relevantQueries, ...optionalQueries]
  const ruleAccessDenied = Boolean(!list && event && [ruleQuery, revisionsQuery].some((query) => query.data === undefined && query.isError && isForbidden(query.error)))
  const errorQuery = relevantQueries.find((query) => query.isError && query.data === undefined)
  const forbidden = Boolean(errorQuery && isForbidden(errorQuery.error))
  const optionalInitialLoadPending = Boolean(ruleId && !list && (!ruleQuery.isFetched || !revisionsQuery.isFetched))
  const loading = optionalInitialLoadPending || relevantQueries.some((query) => query.data === undefined && query.isPending)
  const state: "loading" | "error" | "forbidden" | "ready" = forbidden ? "forbidden" : errorQuery ? "error" : loading ? "loading" : "ready"
  const reportedErrors = useRef(new Set<string>())
  useEffect(() => {
    reportableQueries.forEach((query, index) => {
      if (!query.isError) return
      const errorKey = `${index}:${query.errorUpdatedAt}`
      if (reportedErrors.current.has(errorKey)) return
      reportedErrors.current.add(errorKey)
      notify.error(t("schedules.error.requestFailed"))
    })
  })

  const value = {
    state,
    list,
    ruleId,
    occurrenceDate,
    events: visibleEvents,
    event,
    ruleRevision: effectiveRevision,
    ruleRevisionNumber: rule?.revision || 0,
    ruleAccessDenied,
    projectOptions,
    assigneeOptions,
    assigneeOptionsProjectId: assigneesQuery.data && selectedProjectId === rule?.project_id ? selectedProjectId : undefined,
    timezone,
    userId: userQuery.data?.user_id || "",
    userName: userQuery.data?.user_name || "",
    timezones: timezoneQuery.data?.items || [],
    showSkipped: navigation.showSkipped,
    scheduleQuery: scheduleQuery as UseQueryResult<unknown>,
    ruleQuery: ruleQuery as UseQueryResult<unknown>,
    occurrenceQuery: occurrenceQuery as UseQueryResult<unknown>,
    userQuery: userQuery as UseQueryResult<unknown>,
    timezonesQuery: timezoneQuery as UseQueryResult<unknown>,
    refetch: async () => {
      await queryClient.invalidateQueries({ queryKey: ["schedules"] })
    },
  }

  return <ScheduleAPIDataContext.Provider value={value}>{children}</ScheduleAPIDataContext.Provider>
}

async function fetchProjects(signal: AbortSignal): Promise<RestProjectResponse[]> {
  const items: RestProjectResponse[] = []
  let pageToken: string | undefined
  do {
    const params: GetProjectsParams = { view: "active", page_size: 100, page_token: pageToken }
    const response = await getProjects(params, { signal })
    items.push(...response.items)
    pageToken = response.next_page_token || undefined
  } while (pageToken)
  return items
}

async function fetchOccurrences(range: { from: string; to: string }, projectId: string | undefined, showSkipped: boolean, signal: AbortSignal) {
  const items: RestScheduleOccurrenceResponse[] = []
  let pageToken: string | undefined
  do {
    if (projectId) {
      const params: GetProjectsIdSchedulesParams = {
        from: range.from,
        to: range.to,
        include_skipped: showSkipped,
        page_size: 100,
        page_token: pageToken,
      }
      const response = await getProjectsIdSchedules(projectId, params, { signal })
      items.push(...(response.items || []))
      pageToken = response.next_page_token || undefined
    } else {
      const params: GetSchedulesParams = { from: range.from, to: range.to, include_skipped: showSkipped, page_size: 100, page_token: pageToken }
      const response = await getSchedules(params, { signal })
      items.push(...(response.items || []))
      pageToken = response.next_page_token || undefined
    }
  } while (pageToken)
  return items
}

function isForbidden(error: unknown): boolean {
  return error instanceof ApiError && error.status === 403
}
