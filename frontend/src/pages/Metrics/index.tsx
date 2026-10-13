import { useSearchParams } from "react-router"
import { Link } from "react-router"
import { useGetBillingStatus } from "~/api/generated/billing"
import { useGetOverdueWorkMetrics, useGetTodoCompletionMetrics } from "~/api/generated/metrics"
import { useI18n } from "~/features/i18n/hooks"
import MetricsDashboard from "~/features/metrics/components/MetricsDashboard"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

export default function MetricsPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const i18n = useI18n()
  const billingQuery = useGetBillingStatus({ query: { retry: false } })
  const isPaid = billingQuery.data?.access_plan === "pro" || billingQuery.data?.access_plan === "team"
  const period: 7 | 30 = searchParams.get("days") === "30" ? 30 : 7
  const completionQuery = useGetTodoCompletionMetrics({ days: period }, { query: { retry: false, enabled: isPaid } })
  const overdueQuery = useGetOverdueWorkMetrics({ query: { retry: false, enabled: isPaid } })

  if (!billingQuery.data && billingQuery.isPending) return <p className="text-body">{i18n("metrics.loading")}</p>
  if (billingQuery.error) return <div className={styles.loadError} role="alert"><p className="text-body">{i18n("metrics.loadError")}</p><button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} type="button" onClick={() => { void billingQuery.refetch() }}>{i18n("metrics.retry")}</button></div>
  if (!isPaid) return <section className={styles.loadError} aria-labelledby="metrics-upgrade-title">
    <h1 id="metrics-upgrade-title" className="text-page-title">{i18n("page.metrics.title")}</h1>
    <p className="text-body">{i18n("metrics.paidDescription")}</p>
    <Link to="/billing" className={`${controls.button} ${controls.primaryButton} ${controls.focusRing} text-button-small`}>{i18n("metrics.upgrade")}</Link>
  </section>

  const hasCompletionData = completionQuery.data !== undefined
  const hasOverdueData = overdueQuery.data !== undefined
  const hasData = hasCompletionData && hasOverdueData
  const failed = (completionQuery.isError && !hasCompletionData) || (overdueQuery.isError && !hasOverdueData)
  const loading = (completionQuery.isPending && !hasCompletionData) || (overdueQuery.isPending && !hasOverdueData)

  if (!hasData && loading && !failed) return <p className="text-body">{i18n("metrics.loading")}</p>
  if (!hasData) return <div className={styles.loadError} role="alert">
    <p className="text-body">{i18n("metrics.loadError")}</p>
    <button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} type="button" onClick={() => { void completionQuery.refetch(); void overdueQuery.refetch() }}>{i18n("metrics.retry")}</button>
  </div>

  const completion = completionQuery.data
  const overdue = overdueQuery.data
  const days = (completion.days ?? []).map((day) => ({
    date: day.date ?? "",
    plannedItems: day.planned_items ?? 0,
    completedItems: day.completed_items ?? 0,
    completionRatio: day.completion_ratio ?? null,
  }))
  const overdueGroups = (overdue.projects ?? []).map((project, index) => ({
    id: project.project_id ?? `no-project-${index}`,
    title: project.project_title ?? i18n("metrics.overdue.noProject"),
    count: project.count ?? 0,
  }))
  const isPartialToday = days.some((day) => day.date === completion.end_date)

  return <>
    {(completionQuery.isError || overdueQuery.isError) && <div className={styles.refreshError} role="status">
      <span className="text-caption">{i18n("metrics.loadError")}</span>
      <button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} type="button" onClick={() => { void completionQuery.refetch(); void overdueQuery.refetch() }}>{i18n("metrics.retry")}</button>
    </div>}
    <MetricsDashboard
      period={period}
      plannedItems={completion.total_planned_items ?? 0}
      completedItems={completion.completed_items ?? 0}
      completionRatio={completion.completion_ratio ?? null}
      overdueCount={overdue.total_overdue_count ?? 0}
      isPartialToday={isPartialToday}
      days={days}
      overdueGroups={overdueGroups}
      otherOverdueCount={overdue.other_count ?? 0}
      hasPlannedWork={(completion.total_planned_items ?? 0) > 0}
      onPeriodChange={(nextPeriod) => setSearchParams((current) => {
        const next = new URLSearchParams(current)
        if (nextPeriod === 7) next.delete("days")
        else next.set("days", String(nextPeriod))
        return next
      })}
    />
  </>
}
