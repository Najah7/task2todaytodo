import type { ReactNode } from "react"
import { useContext } from "react"
import { useI18n } from "~/features/i18n/hooks"
import { AllocationAPIDataContext } from "~/features/allocation/providers/AllocationAPIDataProvider/context"
import PageHeading from "~/features/shared/components/PageHeading"

export default function AllocationPlanFormBoundary({ children }: { children: ReactNode }) {
  const data = useContext(AllocationAPIDataContext)
  const i18n = useI18n()
  if (!data) throw new Error("AllocationAPIDataProvider is required")

  if (data.state !== "ready") return <>
    <PageHeading messageKey="weeklyPlan.plan.title" descriptionKey="weeklyPlan.plan.description" />
    <p role={data.state === "error" ? "alert" : undefined} className="text-body">
      {i18n(data.state === "loading" ? "weeklyPlan.loading" : data.state === "forbidden" ? "weeklyPlan.forbidden" : "weeklyPlan.loadError")}
    </p>
    {data.state !== "loading" && <button type="button" onClick={() => { void data.refetch() }}>{i18n("weeklyPlan.retry")}</button>}
  </>

  return children
}
