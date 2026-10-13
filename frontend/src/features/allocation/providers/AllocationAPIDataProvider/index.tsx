import { useEffect, type ReactNode } from "react"
import { useQuery } from "@tanstack/react-query"
import { getGetUsersMeQueryOptions } from "~/api/generated/auth"
import { useGetBillingStatus } from "~/api/generated/billing"
import { getGetAllocationPlansQueryOptions, getGetAllocationRulesQueryOptions, getGetAllocationWeeksWeekStartQueryOptions } from "~/api/generated/allocations"
import { fetchActiveProjects } from "~/features/project/providers/ProjectAPIDataProvider/fetchActiveProjects"
import { ApiError } from "~/api/http"
import { useI18n } from "~/features/i18n/hooks"
import { notify } from "~/features/shared/notification"
import { AllocationAPIDataContext, type AllocationAPIData } from "./context"

export function AllocationAPIDataProvider({ weekStart, loadWeek = true, children }: { weekStart: string; loadWeek?: boolean; children: ReactNode }) {
  const i18n = useI18n()
  const billingQuery = useGetBillingStatus({ query: { retry: false } })
  const userQuery = useQuery({ ...getGetUsersMeQueryOptions(), retry: false })
  const isPaid = billingQuery.data?.access_plan === "pro" || billingQuery.data?.access_plan === "team"
  const rulesQuery = useQuery({ ...getGetAllocationRulesQueryOptions(), retry: false, enabled: isPaid })
  const plansQuery = useQuery({ ...getGetAllocationPlansQueryOptions(), retry: false, enabled: isPaid })
  const weekQuery = useQuery({ ...getGetAllocationWeeksWeekStartQueryOptions(weekStart), retry: false, enabled: loadWeek })
  const projectsQuery = useQuery({ queryKey: ["projects", "active-options"], queryFn: ({ signal }) => fetchActiveProjects(signal), retry: false, enabled: isPaid })
  const errors = [billingQuery.error, userQuery.error, ...(isPaid ? [rulesQuery.error, plansQuery.error, projectsQuery.error] : []), ...(loadWeek ? [weekQuery.error] : [])]
  const forbidden = errors.some((error) => error instanceof ApiError && error.status === 403)
  const hasError = errors.some(Boolean)
  const ready = Boolean(billingQuery.data && userQuery.data && (!loadWeek || weekQuery.data) && (!isPaid || (rulesQuery.data && plansQuery.data && projectsQuery.data)))
  const state: AllocationAPIData["state"] = ready ? "ready" : hasError ? (forbidden ? "forbidden" : "error") : "loading"

  useEffect(() => {
    if (hasError) notify.error(i18n("weeklyPlan.loadError"))
  }, [hasError, i18n])

  const value: AllocationAPIData = {
    state,
    isPaid,
    weekStart,
    timezone: userQuery.data?.timezone || weekQuery.data?.timezone || rulesQuery.data?.timezone || "UTC",
    week: weekQuery.data,
    rules: isPaid ? rulesQuery.data : undefined,
    plans: isPaid ? plansQuery.data : undefined,
    user: userQuery.data,
    projects: isPaid ? projectsQuery.data ?? [] : [],
    weekQuery,
    rulesQuery,
    plansQuery,
    userQuery,
    projectsQuery,
    refetch: async () => {
      await Promise.all([billingQuery.refetch(), ...(loadWeek ? [weekQuery.refetch()] : []), ...(isPaid ? [rulesQuery.refetch(), plansQuery.refetch(), projectsQuery.refetch()] : []), userQuery.refetch()])
    },
  }

  return <AllocationAPIDataContext.Provider value={value}>{children}</AllocationAPIDataContext.Provider>
}
