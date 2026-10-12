import { useI18n } from "~/features/i18n/hooks"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

export type MemberProjectSummary = {
  id: string
  title: string
  memberCount: number
  pendingInvitationCount: number
}

type Props = {
  projects: MemberProjectSummary[]
  selectedId: string
  onSelect: (projectId: string) => void
}

export default function MemberProjectSidebar({ projects, selectedId, onSelect }: Props) {
  const i18n = useI18n()

  return (
    <nav className={`${controls.card} ${styles.panel}`} aria-label={i18n("members.projects")}>
      <h2 className={`${styles.heading} text-label`}>{i18n("members.projects")}</h2>
      {projects.length === 0 ? (
        <p className={`${styles.empty} text-caption`}>{i18n("members.noProjects")}</p>
      ) : (
        <ul className={styles.list}>
          {projects.map((project) => (
            <li key={project.id}>
              <button
                className={`${styles.project} ${controls.focusRing} ${selectedId === project.id ? styles.selected : ""}`}
                type="button"
                aria-current={selectedId === project.id ? "page" : undefined}
                onClick={() => onSelect(project.id)}
              >
                <span className={styles.folder} aria-hidden="true">▱</span>
                <span className={styles.title} title={project.title}>{project.title}</span>
                <span className={styles.counts}>
                  <span aria-label={i18n("members.memberCount", { count: project.memberCount })}>{project.memberCount}</span>
                  {project.pendingInvitationCount > 0 && <span className={styles.pending} aria-label={i18n("members.pendingCount", { count: project.pendingInvitationCount })}>+{project.pendingInvitationCount}</span>}
                </span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </nav>
  )
}
