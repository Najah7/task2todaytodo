import type { RestProjectResponse } from "~/api/generated/projects"
import { useI18n } from "~/features/i18n/hooks"
import { useProjectListActions } from "~/features/project/providers/ProjectList/action"
import { useProjectListNavigation } from "~/features/project/providers/ProjectList/navigation"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

type Props = {
  project: RestProjectResponse
}

export default function ProjectRowActions({ project }: Props) {
  const i18n = useI18n()
  const { state } = useProjectListNavigation()
  const { busy, edit, moveToTrash, restore } = useProjectListActions()
  return (
    <div className={styles.actions} onClick={(event) => event.stopPropagation()}>
      {state.tab === "trash" ? (
        project.can_delete && <button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} type="button" disabled={busy} onClick={() => restore(project)}>{i18n("projects.restore")}</button>
      ) : (
        <>
          {project.can_update && <button className={`${controls.button} ${controls.neutralButton} ${controls.focusRing} text-button-small`} type="button" disabled={busy} onClick={() => edit(project)}>{i18n("projects.edit")}</button>}
          {project.can_delete && (
            <button className={`${styles.iconButton} ${controls.focusRing}`} type="button" aria-label={i18n("projects.moveToTrash", { title: project.title })} disabled={busy} onClick={() => moveToTrash(project)}>
              <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false"><path d="M4 7h16M9 7V4h6v3m3 0-.8 13H6.8L6 7m4 4v5m4-5v5" /></svg>
            </button>
          )}
        </>
      )}
    </div>
  )
}
