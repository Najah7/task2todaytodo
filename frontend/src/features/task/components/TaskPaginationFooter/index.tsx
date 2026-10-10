import { useContext } from "react"
import { useI18n } from "~/features/i18n/hooks"
import { TaskAPIDataContext } from "~/features/task/providers/TaskAPIDataProvider/context"
import { TaskNavigationContext } from "~/features/task/providers/TaskNavigationProvider/context"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

export default function TaskPaginationFooter() {
  const i18n = useI18n()
  const { tasksQuery } = useContext(TaskAPIDataContext)!
  const { state, first, previous, next } = useContext(TaskNavigationContext)!
  const canGoFirst = Boolean(state.pageToken && tasksQuery.data?.items?.length === 0)
  const previousDisabled = !tasksQuery.data?.previous_page_token || tasksQuery.isLoading
  const nextDisabled = !tasksQuery.data?.next_page_token || tasksQuery.isLoading
  return (
    <footer className={styles.footer}>
      {canGoFirst && <button className={`${controls.button} ${controls.neutralButton} ${controls.focusRing} text-body`} type="button" onClick={() => first()}>{i18n("tasks.firstPage")}</button>}
      <button className={`${controls.button} ${controls.neutralButton} ${controls.focusRing} text-body`} type="button" disabled={previousDisabled} onClick={() => previous(tasksQuery.data?.previous_page_token)}>{i18n("tasks.previous")}</button>
      <button className={`${controls.button} ${controls.neutralButton} ${controls.focusRing} text-body`} type="button" disabled={nextDisabled} onClick={() => next(tasksQuery.data?.next_page_token)}>{i18n("tasks.next")}</button>
    </footer>
  )
}
