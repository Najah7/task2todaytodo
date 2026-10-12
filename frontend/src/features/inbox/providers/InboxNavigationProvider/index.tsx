import { useRef, type ReactNode } from "react"
import { useLocation, useNavigate, useSearchParams } from "react-router"
import { inboxListChanges2searchParams, inboxPageTokenChanges2searchParams, inboxSearchParams2state } from "~/features/inbox/converters/inboxSearchParams"
import { InboxNavigationContext, type InboxNavigation } from "./context"

type Props = { children: ReactNode }

export function InboxNavigationProvider({ children }: Props) {
  const [searchParams, setSearchParams] = useSearchParams()
  const navigate = useNavigate()
  const location = useLocation()
  const routeSearch = searchParams.toString()
  const observedRouteSearch = useRef(routeSearch)
  const latestSearchParams = useRef(new URLSearchParams(searchParams))
  const pendingSearches = useRef(new Set<string>())
  if (routeSearch !== observedRouteSearch.current) {
    observedRouteSearch.current = routeSearch
    if (pendingSearches.current.has(routeSearch)) {
      if (routeSearch === latestSearchParams.current.toString()) pendingSearches.current.clear()
    } else {
      latestSearchParams.current = new URLSearchParams(searchParams)
      pendingSearches.current.clear()
    }
  }
  const state = inboxSearchParams2state([searchParams])[0]!
  const currentURL = `${location.pathname}${location.search}`

  function updateSearchParams(next: URLSearchParams, options: { preventScrollReset?: boolean; replace?: boolean }) {
    const normalized = new URLSearchParams(next)
    const serialized = normalized.toString()
    if (serialized === latestSearchParams.current.toString()) return
    latestSearchParams.current = normalized
    pendingSearches.current.add(serialized)
    setSearchParams(normalized, options)
  }

  function changeFilters(changes: { type?: typeof state.type; projectId?: string; title?: string }) {
    updateSearchParams(inboxListChanges2searchParams([{ searchParams: latestSearchParams.current, ...changes }])[0]!, { preventScrollReset: true })
  }

  function editTask(taskId: string) {
    navigate(`/tasks/${encodeURIComponent(taskId)}/edit`, { state: { from: currentURL } })
  }

  function editSchedule(ruleId: string, occurrenceDate: string) {
    navigate(`/schedules/${encodeURIComponent(ruleId)}/${encodeURIComponent(occurrenceDate)}/edit`, { state: { from: currentURL } })
  }

  const navigation: InboxNavigation = {
    state,
    currentURL,
    selectType: (type) => changeFilters({ type }),
    selectProject: (projectId) => changeFilters({ projectId }),
    changeTitle: (title) => updateSearchParams(inboxListChanges2searchParams([{ searchParams: latestSearchParams.current, title }])[0]!, { preventScrollReset: true, replace: true }),
    first: () => updateSearchParams(inboxPageTokenChanges2searchParams([{ searchParams: latestSearchParams.current }])[0]!, { preventScrollReset: true, replace: true }),
    previous: (pageToken) => updateSearchParams(inboxPageTokenChanges2searchParams([{ searchParams: latestSearchParams.current, pageToken }])[0]!, { preventScrollReset: true }),
    next: (pageToken) => updateSearchParams(inboxPageTokenChanges2searchParams([{ searchParams: latestSearchParams.current, pageToken }])[0]!, { preventScrollReset: true }),
    editTask,
    editSchedule,
    editItem: (item) => {
      if (item.type === "task" && item.task) editTask(item.task.id)
      else if (item.type === "schedule" && item.schedule) editSchedule(item.schedule.rule_id, item.schedule.occurrence_date)
    },
  }

  return <InboxNavigationContext.Provider value={navigation}>{children}</InboxNavigationContext.Provider>
}
