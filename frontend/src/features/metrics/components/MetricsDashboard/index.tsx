import { useMemo } from "react"
import { useI18n, useLanguage } from "~/features/i18n/hooks"
import type { MetricsDashboardProps, MetricsDay, OverdueGroup } from "./types"
import styles from "./index.module.css"

function formatPercent(value: number | null, locale: string) {
  return value === null ? "—" : new Intl.NumberFormat(locale, { style: "percent", maximumFractionDigits: 0 }).format(value)
}

function formatDate(date: string, locale: string, options: Intl.DateTimeFormatOptions) {
  return new Intl.DateTimeFormat(locale, options).format(new Date(`${date}T00:00:00`))
}

function SummaryCard({ label, value, detail, emphasized = false }: { label: string; value: string; detail?: string; emphasized?: boolean }) {
  return <article className={`${styles.summaryCard} ${emphasized ? styles.emphasized : ""}`}>
    <p className="text-caption">{label}</p>
    <strong className="text-stat-value">{value}</strong>
    {detail && <span className="text-caption">{detail}</span>}
  </article>
}

function CompletionChart({ days, isPartialToday }: { days: MetricsDay[]; isPartialToday: boolean }) {
  const t = useI18n()
  const { language } = useLanguage()
  const locale = language === "ja" ? "ja-JP" : "en-US"
  const maxCount = Math.max(1, ...days.flatMap((day) => [day.plannedItems, day.completedItems]))
  const chartHeight = 144

  return <section className={styles.panel} aria-labelledby="metrics-chart-title">
    <div className={styles.panelHeader}>
      <h2 className="text-card-title" id="metrics-chart-title">{t("metrics.chart.title")}</h2>
      <div className={styles.legend} aria-label={t("metrics.chart.title")}>
        <span><i className={styles.plannedSwatch} />{t("metrics.chart.planned")}</span>
        <span><i className={styles.completedSwatch} />{t("metrics.chart.completed")}</span>
      </div>
    </div>
    {isPartialToday && <p className={`${styles.partialNote} text-caption`}>{t("metrics.chart.partialToday")}</p>}
    {days.length === 0 ? <p className={`${styles.emptyChart} text-body`}>{t("metrics.chart.noData")}</p> : <>
      <div className={styles.chartScroller}>
        <div className={styles.chart} role="img" aria-label={t("metrics.chart.tableLabel")} style={{ minWidth: days.length > 14 ? `${days.length * 2.4}rem` : undefined }}>
          {days.map((day) => {
            const plannedHeight = day.plannedItems / maxCount * chartHeight
            const completedHeight = day.completedItems / maxCount * chartHeight
            return <div className={styles.chartDay} key={day.date}>
              <div className={styles.barArea}>
                {day.completionRatio === null && <span className={styles.noDataMarker} aria-hidden="true" />}
                <span className={`${styles.bar} ${styles.plannedBar}`} style={{ height: `${plannedHeight}px` }} />
                <span className={`${styles.bar} ${styles.completedBar}`} style={{ height: `${completedHeight}px` }} />
              </div>
              <span className={`${styles.dayLabel} text-chart-date`}>{formatDate(day.date, locale, { weekday: "short" })}</span>
              <span className={`${styles.dateLabel} text-chart-date`}>{formatDate(day.date, locale, { month: "numeric", day: "numeric" })}</span>
            </div>
          })}
        </div>
      </div>
      <table className={styles.detailTable}>
        <caption className="sr-only">{t("metrics.chart.tableLabel")}</caption>
        <thead><tr><th scope="col">{t("metrics.chart.date")}</th><th scope="col">{t("metrics.chart.planned")}</th><th scope="col">{t("metrics.chart.completed")}</th><th scope="col">{t("metrics.chart.rate")}</th></tr></thead>
        <tbody>{days.map((day) => <tr key={day.date}>
          <th scope="row">{formatDate(day.date, locale, { dateStyle: "medium" })}</th>
          <td>{day.plannedItems}</td><td>{day.completedItems}</td><td>{formatPercent(day.completionRatio, locale)}</td>
        </tr>)}</tbody>
      </table>
    </>}
  </section>
}

function OverduePanel({ groups, totalCount, otherCount }: { groups: OverdueGroup[]; totalCount: number; otherCount: number }) {
  const t = useI18n()
  const rows = useMemo(() => {
    const result = [...groups]
    if (otherCount > 0) result.push({ id: "other", title: t("metrics.overdue.other"), count: otherCount })
    return result
  }, [groups, otherCount, t])

  return <section className={styles.panel} aria-labelledby="metrics-overdue-title">
    <div className={styles.panelHeader}>
      <h2 className="text-card-title" id="metrics-overdue-title">{t("metrics.overdue.title")}</h2>
      <span className="text-caption">{totalCount}</span>
    </div>
    {rows.length === 0 ? <p className={`${styles.overdueEmpty} text-body`}>{t("metrics.overdue.empty")}</p> : <table className={styles.overdueTable}>
      <thead><tr><th scope="col">{t("metrics.overdue.project")}</th><th scope="col">{t("metrics.overdue.count")}</th></tr></thead>
      <tbody>{rows.map((row) => <tr key={row.id}><th scope="row">{row.title}</th><td>{row.count}</td></tr>)}</tbody>
    </table>}
  </section>
}

export default function MetricsDashboard({ period, plannedItems, completedItems, completionRatio, overdueCount, isPartialToday, days, overdueGroups, otherOverdueCount, hasPlannedWork, onPeriodChange }: MetricsDashboardProps) {
  const t = useI18n()
  const { language } = useLanguage()
  const locale = language === "ja" ? "ja-JP" : "en-US"
  const periodLabel = t(`metrics.period.${period}` as "metrics.period.7" | "metrics.period.30")

  return <div className={styles.dashboard}>
    <header className={styles.heading}>
      <div><h1 className="text-page-title">{t("page.metrics.title")}</h1><p className="text-description">{t("metrics.description")}</p></div>
      <div className={styles.periodSwitch} role="group" aria-label={t("metrics.description")}>
        {([7, 30] as const).map((value) => <button key={value} className={`${styles.periodButton} ${period === value ? styles.periodSelected : ""}`} type="button" aria-pressed={period === value} onClick={() => onPeriodChange(value)}>{t(`metrics.period.${value}` as "metrics.period.7" | "metrics.period.30")}</button>)}
      </div>
    </header>
    <section className={styles.summaryGrid} aria-label={periodLabel}>
      <SummaryCard label={t("metrics.summary.rate")} value={formatPercent(completionRatio, locale)} />
      <SummaryCard label={t("metrics.summary.completed")} value={String(completedItems)} />
      <SummaryCard label={t("metrics.summary.planned")} value={String(plannedItems)} />
      <SummaryCard label={t("metrics.summary.overdue")} value={String(overdueCount)} emphasized={overdueCount > 0} />
    </section>
    {!hasPlannedWork && <div className={styles.noPlan} role="status"><strong className="text-card-title">{t("metrics.empty.title")}</strong><p className="text-body">{t("metrics.empty.description")}</p></div>}
    <div className={styles.contentGrid}>
      <CompletionChart days={days} isPartialToday={isPartialToday} />
      <OverduePanel groups={overdueGroups} totalCount={overdueCount} otherCount={otherOverdueCount} />
    </div>
  </div>
}
