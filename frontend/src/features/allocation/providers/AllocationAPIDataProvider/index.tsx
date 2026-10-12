import { useEffect, type ReactNode } from "react"
import { useQuery } from "@tanstack/react-query"
import { getGetUsersMeQueryOptions } from "~/api/generated/auth"
import { getGetAllocationPlansQueryOptions, getGetAllocationRulesQueryOptions, getGetAllocationWeeksWeekStartQueryOptions } from "~/api/generated/allocations"
import { fetchActiveProjects } from "~/features/project/providers/ProjectAPIDataProvider/fetchActiveProjects"
import { ApiError } from "~/api/http"
import { useI18n } from "~/features/i18n/hooks"
import { notify } from "~/features/shared/notification"
import { AllocationAPIDataContext, type AllocationAPIData } from "./context"

export function AllocationAPIDataProvider({ weekStart, loadWeek = true, children }: { weekStart: string; loadWeek?: boolean; children: ReactNode }) {
  const i18n = useI18n()
  const userQuery = useQuery({ ...getGetUsersMeQueryOptions(), retry: false })
  const rulesQuery = useQuery({ ...getGetAllocationRulesQueryOptions(), retry: false })
  const plansQuery = useQuery({ ...getGetAllocationPlansQueryOptions(), retry: false })
  const weekQuery = useQuery({ ...getGetAllocationWeeksWeekStartQueryOptions(weekStart), retry: false, enabled: loadWeek })
  const projectsQuery = useQuery({ queryKey: ["projects", "active-options"], queryFn: ({ signal }) => fetchActiveProjects(signal), retry: false })
  const errors = [userQuery.error, rulesQuery.error, plansQuery.error, ...(loadWeek ? [weekQuery.error] : []), projectsQuery.error]
  const forbidden = errors.some((error) => error instanceof ApiError && error.status === 403)
  const hasError = errors.some(Boolean)
  const ready = Boolean(userQuery.data && rulesQuery.data && plansQuery.data && (!loadWeek || weekQuery.data) && projectsQuery.data)
  const state: AllocationAPIData["state"] = ready ? "ready" : hasError ? (forbidden ? "forbidden" : "error") : "loading"

  useEffect(() => {
    if (hasError) notify.error(i18n("weeklyPlan.loadError"))
  }, [hasError, i18n])

  const value: AllocationAPIData = {
    state,
    weekStart,
    timezone: userQuery.data?.timezone || weekQuery.data?.timezone || rulesQuery.data?.timezone || "UTC",
    week: weekQuery.data,
    rules: rulesQuery.data,
    plans: plansQuery.data,
    user: userQuery.data,
    projects: projectsQuery.data ?? [],
    weekQuery,
    rulesQuery,
    plansQuery,
    userQuery,
    projectsQuery,
    refetch: async () => {
      await Promise.all([...(loadWeek ? [weekQuery.refetch()] : []), rulesQuery.refetch(), plansQuery.refetch(), userQuery.refetch(), projectsQuery.refetch()])
    },
  }

  return <AllocationAPIDataContext.Provider value={value}>{children}</AllocationAPIDataContext.Provider>
}
