import { useContext } from "react"
import { useI18n } from "~/features/i18n/hooks"
import { GetProjectsStatus } from "~/api/generated/projects"
import { ProjectAPIDataContext } from "~/features/project/providers/ProjectAPIDataProvider/context"
import { ProjectNavigationContext } from "~/features/project/providers/ProjectNavigationProvider/context"
import { projectTabLabels } from "./constants"
import type { ProjectTab } from "~/features/project/providers/ProjectNavigationProvider/context"
import styles from "./index.module.css"

const projectTabs: ProjectTab[] = [
  GetProjectsStatus.in_progress,
  GetProjectsStatus.pending,
  GetProjectsStatus.done,
  GetProjectsStatus.open,
  GetProjectsStatus.waiting_on_others,
  "trash",
]

export default function ProjectStatusTab() {
  const i18n = useI18n()
  const { projectsQuery } = useContext(ProjectAPIDataContext)!
  const { state, selectTab } = useContext(ProjectNavigationContext)!
  const summary = projectsQuery.data?.summary
  return (
    <nav className={styles.tabs} aria-label={i18n("projects.status.select", { title: i18n("page.projects.title") })}>
      {projectTabs.map((tab) => {
        const count = tab === "trash" ? summary?.trash_count : summary?.status_counts[tab]
        return (
          <button key={tab} className={`${styles.tab} ${state.tab === tab ? styles.activeTab : ""} text-body`} type="button" aria-current={state.tab === tab ? "page" : undefined} onClick={() => selectTab(tab)}>
            <span>{i18n(projectTabLabels[tab])}</span>
            <span className={`${styles.tabCount} text-numeric`}>{count ?? "—"}</span>
          </button>
        )
      })}
    </nav>
  )
}
