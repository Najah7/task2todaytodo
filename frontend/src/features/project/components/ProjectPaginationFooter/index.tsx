import { useContext, useEffect } from "react"
import { useI18n } from "~/features/i18n/hooks"
import { ProjectAPIDataContext } from "~/features/project/providers/ProjectAPIDataProvider/context"
import { ProjectActionContext } from "~/features/project/providers/ProjectActionProvider/context"
import { ProjectNavigationContext } from "~/features/project/providers/ProjectNavigationProvider/context"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

export default function ProjectPaginationFooter() {
  const i18n = useI18n()
  const { projectsQuery } = useContext(ProjectAPIDataContext)!
  const { state, first, previous, next } = useContext(ProjectNavigationContext)!
  const { busy } = useContext(ProjectActionContext)!
  const response = projectsQuery.data
  const showFirstPage = Boolean(state.pageToken && response?.items.length === 0 && !response.previous_page_token)
  const previousDisabled = !response?.previous_page_token || busy || projectsQuery.isLoading
  const nextDisabled = !response?.next_page_token || busy || projectsQuery.isLoading

  useEffect(() => {
    if (!state.pageToken || projectsQuery.isFetching || !response || response.items.length > 0 || response.previous_page_token) return
    first({ replace: true })
  }, [first, projectsQuery.isFetching, response, state.pageToken])
  const buttonClass = `${controls.button} ${controls.neutralButton} ${controls.focusRing} text-body`

  return (
    <footer className={styles.pagination}>
      {showFirstPage && (
        <button className={buttonClass} type="button" onClick={() => first()}>{i18n("projects.firstPage")}</button>
      )}
      <button className={buttonClass} type="button" disabled={previousDisabled} onClick={() => previous(response?.previous_page_token)}>{i18n("projects.previous")}</button>
      <button className={buttonClass} type="button" disabled={nextDisabled} onClick={() => next(response?.next_page_token)}>{i18n("projects.next")}</button>
    </footer>
  )
}
