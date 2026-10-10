import type { ReactNode } from "react"
import { useLocation, useNavigate, useSearchParams } from "react-router"
import { taskFiltersChanges2searchParams, taskPageTokenChanges2searchParams, taskSearchParams2states, taskSortChanges2searchParams, taskStatusChanges2searchParams } from "~/features/task/converters/taskSearchParams"
import { TaskNavigationContext, type TaskFilterView, type TaskNavigation } from "./context"

type Props = { children: ReactNode; defaultReturnTo?: string }

export function TaskNavigationProvider({ children, defaultReturnTo = "/tasks" }: Props) {
  const [searchParams, setSearchParams] = useSearchParams()
  const navigate = useNavigate()
  const location = useLocation()
  const state = taskSearchParams2states([searchParams])[0]!
  const returnTo = taskReturnPath(location.state, defaultReturnTo)
  const from = `${location.pathname}${location.search}`

  const navigation: TaskNavigation = {
    state,
    create: () => navigate(`/tasks/new${state.projectId ? `?project_id=${encodeURIComponent(state.projectId)}` : ""}`, { state: { from } }),
    edit: (taskId) => navigate(`/tasks/${encodeURIComponent(taskId)}/edit`, { state: { from } }),
    cancel: () => navigate(returnTo),
    afterSave: () => navigate(returnTo, { replace: true }),
    selectStatus: (value) => setSearchParams(taskStatusChanges2searchParams([{ searchParams, status: value as typeof state.status }])[0]!, { preventScrollReset: true }),
    changeFilter: (next: TaskFilterView) => {
      const nextParams = taskFiltersChanges2searchParams([{
        searchParams,
        projectId: next.projectId,
        dueFilter: next.dueFilter as typeof state.dueFilter,
        title: next.title,
      }])[0]!
      setSearchParams(nextParams, { preventScrollReset: true, replace: next.title !== state.title })
    },
    changeSort: (sort) => {
      const [sortBy, sortOrder] = sort.split(":") as [NonNullable<typeof state.sortBy>, typeof state.sortOrder]
      setSearchParams(taskSortChanges2searchParams([{ searchParams, sortBy, sortOrder }])[0]!, { preventScrollReset: true })
    },
    first: (options) => setSearchParams(taskPageTokenChanges2searchParams([{ searchParams }])[0]!, { preventScrollReset: true, replace: options?.replace }),
    previous: (pageToken) => setSearchParams(taskPageTokenChanges2searchParams([{ searchParams, pageToken }])[0]!, { preventScrollReset: true }),
    next: (pageToken) => setSearchParams(taskPageTokenChanges2searchParams([{ searchParams, pageToken }])[0]!, { preventScrollReset: true }),
  }

  return <TaskNavigationContext.Provider value={navigation}>{children}</TaskNavigationContext.Provider>
}

function taskReturnPath(state: unknown, fallback: string): string {
  if (state && typeof state === "object" && "from" in state && typeof state.from === "string" && state.from.startsWith("/tasks")) return state.from
  return fallback
}
