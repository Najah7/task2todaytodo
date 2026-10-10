import { useContext } from "react"
import PageHeading from "~/features/shared/components/PageHeading"
import { useI18n } from "~/features/i18n/hooks"
import TaskListTable from "~/features/task/components/TaskListTable"
import TaskListToolbar from "~/features/task/components/TaskListToolbar"
import TaskPaginationFooter from "~/features/task/components/TaskPaginationFooter"
import TaskStatusTabs from "~/features/task/components/TaskStatusTabs"
import { TaskActionProvider } from "~/features/task/providers/TaskActionProvider"
import { TaskActionContext } from "~/features/task/providers/TaskActionProvider/context"
import { TaskAPIDataProvider } from "~/features/task/providers/TaskAPIDataProvider"
import { TaskAPIDataContext } from "~/features/task/providers/TaskAPIDataProvider/context"
import { TaskNavigationProvider } from "~/features/task/providers/TaskNavigationProvider"
import { TaskNavigationContext } from "~/features/task/providers/TaskNavigationProvider/context"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

export default function TaskList() {
  const i18n = useI18n()
  return (
    <TaskAPIDataProvider list>
      <TaskActionProvider>
        <TaskNavigationProvider>
          <section className={styles.page} aria-labelledby="tasks-heading">
            <TaskListHeader />
            <TaskListOptionsError />
            <TaskStatusTabs />
            <section className={`${controls.card} ${styles.card}`} aria-label={i18n("page.tasks.title")}>
              <TaskListToolbar />
              <TaskListTable />
              <TaskPaginationFooter />
            </section>
          </section>
        </TaskNavigationProvider>
      </TaskActionProvider>
    </TaskAPIDataProvider>
  )
}

function TaskListHeader() {
  const i18n = useI18n()
  const { projectsQuery } = useContext(TaskAPIDataContext)!
  const { create } = useContext(TaskNavigationContext)!
  return (
    <header className={styles.header}>
      <PageHeading id="tasks-heading" messageKey="page.tasks.title" descriptionKey="tasks.description" />
      <button className={`${controls.button} ${controls.primaryButton} ${controls.focusRing} text-button`} type="button" disabled={projectsQuery.isLoading || Boolean(projectsQuery.error && !projectsQuery.data)} onClick={create}>{i18n("tasks.create")}</button>
    </header>
  )
}

function TaskListOptionsError() {
  const i18n = useI18n()
  const { projectsQuery } = useContext(TaskAPIDataContext)!
  const { retryOptions } = useContext(TaskActionContext)!
  if (!projectsQuery.error || projectsQuery.data) return null
  return <div className={styles.message} role="alert"><p className="text-body">{i18n("tasks.options.loadError")}</p><button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} type="button" onClick={() => void retryOptions()}>{i18n("tasks.retry")}</button></div>
}
