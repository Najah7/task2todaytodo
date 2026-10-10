import { useI18n } from "~/features/i18n/hooks"
import { useTaskListData } from "~/features/task/providers/TaskList/data"
import { useTaskListNavigation } from "~/features/task/providers/TaskList/navigation"
import styles from "./index.module.css"

export default function TaskStatusTabs() {
  const i18n = useI18n()
  const { tabs } = useTaskListData()
  const { selectStatus } = useTaskListNavigation()
  return (
    <nav className={styles.tabs} aria-label={i18n("tasks.status.select")}>
      {tabs.map((tab) => (
        <button
          key={tab.value}
          className={`${styles.tab} ${tab.selected ? styles.activeTab : ""} text-body`}
          type="button"
          aria-current={tab.selected ? "page" : undefined}
          onClick={() => selectStatus(tab.value)}
        >
          <span>{i18n(tab.label)}</span>
          <span className={`${styles.count} text-numeric`}>{tab.count}</span>
        </button>
      ))}
    </nav>
  )
}
