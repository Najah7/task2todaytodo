import { useNavigate } from "react-router"
import { useI18n } from "~/features/i18n/hooks"
import ProjectCreateButton from "~/features/project/components/ProjectCreateButton"
import ProjectPaginationFooter from "~/features/project/components/ProjectPaginationFooter"
import ProjectStats from "~/features/project/components/ProjectStats"
import ProjectStatusTab from "~/features/project/components/ProjectStatusTab"
import ProjectTable from "~/features/project/components/ProjectTable"
import { ProjectActionProvider } from "~/features/project/providers/ProjectActionProvider"
import { ProjectAPIDataProvider } from "~/features/project/providers/ProjectAPIDataProvider"
import { ProjectNavigationProvider } from "~/features/project/providers/ProjectNavigationProvider"
import PageHeading from "~/features/shared/components/PageHeading"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

export default function ProjectList() {
  const i18n = useI18n()
  const navigate = useNavigate()

  return (
    <ProjectNavigationProvider>
      <ProjectAPIDataProvider list includeOptions>
        <ProjectActionProvider>
          <section className={styles.page} aria-labelledby="projects-heading">
            <header className={styles.header}>
              <PageHeading id="projects-heading" messageKey="page.projects.title" descriptionKey="projects.description" />
              <ProjectCreateButton label={i18n("projects.create")} onClick={() => navigate("/projects/new")} />
            </header>
            <ProjectStatusTab />
            <section className={`${controls.card} ${styles.listCard}`} aria-label={i18n("page.projects.title")}>
              <ProjectStats />
              <ProjectTable />
              <ProjectPaginationFooter />
            </section>
          </section>
        </ProjectActionProvider>
      </ProjectAPIDataProvider>
    </ProjectNavigationProvider>
  )
}
