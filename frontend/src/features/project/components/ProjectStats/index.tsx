import { useI18n, useLanguage } from "~/features/i18n/hooks"
import { useProjectListData } from "~/features/project/providers/ProjectList/data"
import { useProjectListNavigation } from "~/features/project/providers/ProjectList/navigation"
import { projectTabLabels } from "~/features/project/constants"
import styles from "./index.module.css"

export default function ProjectStats() {
  const i18n = useI18n()
  const { language } = useLanguage()
  const { summary } = useProjectListData()
  const { state } = useProjectListNavigation()
  const values = [
    { label: i18n(projectTabLabels[state.tab]), value: summary?.total_count },
    { label: i18n("projects.summary.dueSoon"), value: summary?.due_soon_count },
    { label: i18n("projects.summary.overdue"), value: summary?.overdue_count, danger: true },
    { label: i18n("projects.summary.today"), value: i18n("projects.summary.placeholder") },
  ]
  return (
    <div className={styles.summary}>
      {values.map(({ label, value, danger }) => (
        <div className={styles.summaryItem} key={label}>
          <span className="text-label">{label}</span>
          <strong className={`${danger ? styles.summaryDanger : ""} text-stat-value`}>
            {typeof value === "number"
              ? i18n("projects.count", { count: new Intl.NumberFormat(language === "ja" ? "ja-JP" : "en-US").format(value) })
              : value ?? "—"}
          </strong>
        </div>
      ))}
    </div>
  )
}
