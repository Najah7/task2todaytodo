import { useI18n } from "~/features/i18n/hooks"
import { useTaskListData } from "~/features/task/providers/TaskList/data"
import { useTaskListNavigation } from "~/features/task/providers/TaskList/navigation"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

export default function TaskPaginationFooter() {
  const i18n = useI18n()
  const { canGoFirst, previousDisabled, nextDisabled } = useTaskListData()
  const { first, previous, next } = useTaskListNavigation()
  return (
    <footer className={styles.footer}>
      {canGoFirst && <button className={`${controls.button} ${controls.neutralButton} ${controls.focusRing} text-body`} type="button" onClick={first}>{i18n("tasks.firstPage")}</button>}
      <button className={`${controls.button} ${controls.neutralButton} ${controls.focusRing} text-body`} type="button" disabled={previousDisabled} onClick={previous}>{i18n("tasks.previous")}</button>
      <button className={`${controls.button} ${controls.neutralButton} ${controls.focusRing} text-body`} type="button" disabled={nextDisabled} onClick={next}>{i18n("tasks.next")}</button>
    </footer>
  )
}
