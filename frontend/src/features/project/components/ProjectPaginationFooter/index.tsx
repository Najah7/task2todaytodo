import { useI18n } from "~/features/i18n/hooks"
import { useProjectListNavigation } from "~/features/project/providers/ProjectList/navigation"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

export default function ProjectPaginationFooter() {
  const i18n = useI18n()
  const { showFirstPage, previousDisabled, nextDisabled, onFirstPage, onPrevious, onNext } = useProjectListNavigation()
  const buttonClass = `${controls.button} ${controls.neutralButton} ${controls.focusRing} text-body`

  return (
    <footer className={styles.pagination}>
      {showFirstPage && (
        <button className={buttonClass} type="button" onClick={onFirstPage}>{i18n("projects.firstPage")}</button>
      )}
      <button className={buttonClass} type="button" disabled={previousDisabled} onClick={onPrevious}>{i18n("projects.previous")}</button>
      <button className={buttonClass} type="button" disabled={nextDisabled} onClick={onNext}>{i18n("projects.next")}</button>
    </footer>
  )
}
