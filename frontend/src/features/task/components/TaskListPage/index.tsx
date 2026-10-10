import PageHeading from "~/features/shared/components/PageHeading"
import { useI18n } from "~/features/i18n/hooks"
import { useTaskListData } from "~/features/task/providers/TaskList/data"
import { useTaskListActions } from "~/features/task/providers/TaskList/action"
import controls from "~/styles/controls.module.css"
import TaskListTable from "~/features/task/components/TaskListTable"
import TaskListToolbar from "~/features/task/components/TaskListToolbar"
import TaskPaginationFooter from "~/features/task/components/TaskPaginationFooter"
import TaskStatusTabs from "~/features/task/components/TaskStatusTabs"
import styles from "./index.module.css"

export default function TaskListPage() {
  const i18n = useI18n()
  const { loading, error, rows, optionsLoading, optionsError } = useTaskListData()
  const { create, retry, retryOptions } = useTaskListActions()
  return (
    <section className={styles.page} aria-labelledby="tasks-heading">
      <header className={styles.header}>
        <PageHeading id="tasks-heading" messageKey="page.tasks.title" descriptionKey="tasks.description" />
        <button className={`${controls.button} ${controls.primaryButton} ${controls.focusRing} text-button`} type="button" disabled={optionsLoading || optionsError} onClick={create}>{i18n("tasks.create")}</button>
      </header>
      {optionsError && <div className={styles.message} role="alert"><p className="text-body">{i18n("tasks.options.loadError")}</p><button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} type="button" onClick={retryOptions}>{i18n("tasks.retry")}</button></div>}
      <TaskStatusTabs />
      <section className={`${controls.card} ${styles.card}`} aria-label={i18n("page.tasks.title")}>
        <TaskListToolbar />
        {loading ? (
          <p className={`${styles.message} text-body`} role="status">{i18n("tasks.loading")}</p>
        ) : error ? (
          <div className={styles.message} role="alert">
            <p className="text-body">{i18n("tasks.loadError")}</p>
            <button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} type="button" onClick={retry}>{i18n("tasks.retry")}</button>
          </div>
        ) : rows.length === 0 ? (
          <p className={`${styles.message} text-body`}>{i18n("tasks.empty")}</p>
        ) : (
          <TaskListTable />
        )}
        <TaskPaginationFooter />
      </section>
    </section>
  )
}
