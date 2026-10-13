import { Link, useSearchParams } from "react-router"
import { useContext } from "react"
import { useQuery } from "@tanstack/react-query"
import { getGetUsersMeQueryOptions } from "~/api/generated/auth"
import { dateInTimezone, isDateKey, mondayOf } from "~/features/allocation/lib/week"
import { AllocationAPIDataProvider } from "~/features/allocation/providers/AllocationAPIDataProvider"
import { AllocationActionProvider } from "~/features/allocation/providers/AllocationActionProvider"
import { AllocationNavigationProvider } from "~/features/allocation/providers/AllocationNavigationProvider"
import PageHeading from "~/features/shared/components/PageHeading"
import AllocationPlanFormBoundary from "~/features/allocation/components/AllocationPlanFormBoundary"
import AllocationPlanForm from "~/features/allocation/components/AllocationPlanForm"
import { useAllocationPlanForm } from "~/features/allocation/hooks/useAllocationPlanForm"
import { AllocationAPIDataContext } from "~/features/allocation/providers/AllocationAPIDataProvider/context"
import { useI18n } from "~/features/i18n/hooks"

export default function WeeklyPlanEditPage() {
  const [params] = useSearchParams()
  const userQuery = useQuery({ ...getGetUsersMeQueryOptions(), retry: false })
  const requested = params.get("week_start")
  const timezone = userQuery.data?.timezone || Intl.DateTimeFormat().resolvedOptions().timeZone
  const today = dateInTimezone(new Date(), timezone)
  const weekStart = requested && isDateKey(requested) ? mondayOf(requested) : mondayOf(today)
  return <AllocationAPIDataProvider weekStart={weekStart} loadWeek={false}>
    <AllocationNavigationProvider weekStart={weekStart}><AllocationActionProvider><AllocationPlanFormBoundary><PlanPageContents /></AllocationPlanFormBoundary></AllocationActionProvider></AllocationNavigationProvider>
  </AllocationAPIDataProvider>
}

function PlanPageContents() {
  const data = useContext(AllocationAPIDataContext)
  const i18n = useI18n()
  if (!data) throw new Error("AllocationAPIDataProvider is required")
  if (!data.isPaid) return <>
    <PageHeading messageKey="weeklyPlan.plan.title" descriptionKey="weeklyPlan.plan.description" />
    <p className="text-body">{i18n("weeklyPlan.activeTime.freeDescription")}</p>
    <Link to="/billing">{i18n("weeklyPlan.upgrade")}</Link>
  </>
  return <PlanForm />
}

function PlanForm() {
  const form = useAllocationPlanForm()
  return <>
    <PageHeading messageKey="weeklyPlan.plan.title" descriptionKey="weeklyPlan.plan.description" />
    <AllocationPlanForm form={form.form} {...form.viewProps} />
  </>
}
