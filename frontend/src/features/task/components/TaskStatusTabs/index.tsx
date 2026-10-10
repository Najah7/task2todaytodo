import { useContext } from "react"
import { useI18n } from "~/features/i18n/hooks"
import { TaskAPIDataContext } from "~/features/task/providers/TaskAPIDataProvider/context"
import { TaskNavigationContext } from "~/features/task/providers/TaskNavigationProvider/context"
import { taskListResponses2statusTabs } from "~/features/task/converters/taskListResponses2summaryViews"
import styles from "./index.module.css"

export default function TaskStatusTabs() {
  const i18n = useI18n()
  const { tasksQuery } = useContext(TaskAPIDataContext)!
  const { state, selectStatus } = useContext(TaskNavigationContext)!
  const tabs = taskListResponses2statusTabs([tasksQuery.data], [state.status])[0]!
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
