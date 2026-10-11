import { useContext } from "react"
import { useI18n } from "~/features/i18n/hooks"
import { InboxAPIDataContext } from "~/features/inbox/providers/InboxAPIDataProvider/context"
import { InboxNavigationContext } from "~/features/inbox/providers/InboxNavigationProvider/context"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

export default function InboxToolbar() {
  const i18n = useI18n()
  const { projects } = useContext(InboxAPIDataContext)!
  const { state, selectProject, changeTitle } = useContext(InboxNavigationContext)!
  return <div className={styles.toolbar}>
    <label className={styles.projectFilter}>
      <span className="text-label">{i18n("inbox.filter.project")}</span>
      <select className={`${controls.select} ${controls.focusRing} ${styles.select} text-body`} aria-label={i18n("inbox.filter.project")} value={state.projectId} onChange={(event) => selectProject(event.target.value)}>
        <option value="all">{i18n("inbox.filter.allProjects")}</option>
        <option value="no-project">{i18n("inbox.filter.noProject")}</option>
        {(projects ?? []).map((project) => <option key={project.id} value={project.id}>{project.title}</option>)}
      </select>
    </label>
    <label className={styles.search}>
      <span className="text-label">{i18n("inbox.filter.title")}</span>
      <input className={`${controls.input} ${styles.searchInput} text-body`} type="search" aria-label={i18n("inbox.filter.title")} value={state.title} placeholder={i18n("inbox.filter.title")} onChange={(event) => changeTitle(event.target.value)} />
    </label>
  </div>
}
