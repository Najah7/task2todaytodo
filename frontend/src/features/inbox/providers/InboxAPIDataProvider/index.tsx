import { useEffect, useMemo, useRef, type ReactNode } from "react"
import { useQuery } from "@tanstack/react-query"
import { useSearchParams } from "react-router"
import { getGetUsersMeQueryOptions } from "~/api/generated/auth"
import { useListInbox } from "~/api/generated/inbox"
import { getGetProjectsQueryKey, GetProjectsView } from "~/api/generated/projects"
import type { GetProjectsParams } from "~/api/generated/projects"
import { ApiError } from "~/api/http"
import { useI18n } from "~/features/i18n/hooks"
import { notify } from "~/features/shared/notification"
import { inboxSearchParams2state, inboxState2apiParams } from "~/features/inbox/converters/inboxSearchParams"
import { InboxAPIDataContext, type InboxAPIDataLoadState } from "./context"
import { fetchActiveProjects } from "~/features/project/providers/ProjectAPIDataProvider/fetchActiveProjects"

type Props = { children: ReactNode }

export function InboxAPIDataProvider({ children }: Props) {
  const [searchParams] = useSearchParams()
  const t = useI18n()
  const listState = useMemo(() => inboxSearchParams2state([searchParams])[0]!, [searchParams])
  const params = useMemo(() => inboxState2apiParams([listState])[0]!, [listState])
  const inboxQuery = useListInbox(params, { query: { retry: false } })
  const projectParams: GetProjectsParams = { view: GetProjectsView.active, page_size: 100 }
  const projectsQuery = useQuery({
    queryKey: [...getGetProjectsQueryKey(projectParams), "all-pages"],
    queryFn: ({ signal }) => fetchActiveProjects(signal),
    staleTime: 5 * 60 * 1000,
    retry: false,
  })
  const userQuery = useQuery({ ...getGetUsersMeQueryOptions(), retry: false })

  const requiredQueries = [inboxQuery, projectsQuery, userQuery]
  const forbidden = requiredQueries.some((query) => query.data === undefined && query.isError && isForbidden(query.error))
  const error = requiredQueries.some((query) => query.data === undefined && query.isError)
  const loading = requiredQueries.some((query) => query.data === undefined && query.isPending)
  const state: InboxAPIDataLoadState = forbidden ? "forbidden" : error ? "error" : loading ? "loading" : "ready"

  const reportedErrors = useRef(new Set<string>())
  useEffect(() => {
    ;[
      [inboxQuery.isError, inboxQuery.errorUpdatedAt],
      [projectsQuery.isError, projectsQuery.errorUpdatedAt],
      [userQuery.isError, userQuery.errorUpdatedAt],
    ].forEach(([isError, errorUpdatedAt], index) => {
      if (!isError) return
      const errorKey = `${index}:${errorUpdatedAt}`
      if (reportedErrors.current.has(errorKey)) return
      reportedErrors.current.add(errorKey)
      notify.error(t("inbox.loadError"))
    })
  }, [inboxQuery.isError, inboxQuery.errorUpdatedAt, projectsQuery.isError, projectsQuery.errorUpdatedAt, userQuery.isError, userQuery.errorUpdatedAt, t])

  const value = {
    state,
    inbox: inboxQuery.data,
    inboxQuery,
    projects: (projectsQuery.data ?? []).map((project) => ({ id: project.id, title: project.title })),
    projectsQuery,
    user: userQuery.data,
    userQuery,
    timezone: userQuery.data?.timezone || "",
    userId: userQuery.data?.user_id || "",
  }

  return <InboxAPIDataContext.Provider value={value}>{children}</InboxAPIDataContext.Provider>
}

function isForbidden(error: unknown): boolean {
  return error instanceof ApiError && (error.status === 401 || error.status === 403)
}
