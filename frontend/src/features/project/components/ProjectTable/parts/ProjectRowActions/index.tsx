import { useContext, useState } from "react"
import type { RestProjectResponse } from "~/api/generated/projects"
import { useI18n } from "~/features/i18n/hooks"
import { ProjectActionContext } from "~/features/project/providers/ProjectActionProvider/context"
import { ProjectNavigationContext } from "~/features/project/providers/ProjectNavigationProvider/context"
import ConfirmationDialog from "~/features/shared/components/ConfirmationDialog"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

type Props = {
  project: RestProjectResponse
}

export default function ProjectRowActions({ project }: Props) {
  const i18n = useI18n()
  const [trashOpen, setTrashOpen] = useState(false)
  const { state, edit, selectTab } = useContext(ProjectNavigationContext)!
  const { busy, trashBusy, trash, restore } = useContext(ProjectActionContext)!

  async function confirmTrash() {
    try {
      await trash(project)
    } catch {
      // The action reports its own failure toast.
    } finally {
      setTrashOpen(false)
    }
  }

  async function restoreProject() {
    const restored = await restore(project)
    if (restored) selectTab(restored.status)
  }

  return (
    <div className={styles.actions} onClick={(event) => event.stopPropagation()}>
      {state.tab === "trash" ? (
        project.can_delete && <button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} type="button" disabled={busy} onClick={() => void restoreProject()}>{i18n("projects.restore")}</button>
      ) : (
        <>
          {project.can_update && <button className={`${controls.button} ${controls.neutralButton} ${controls.focusRing} text-button-small`} type="button" disabled={busy} onClick={() => edit(project.id)}>{i18n("projects.edit")}</button>}
          {project.can_delete && (
            <button className={`${styles.iconButton} ${controls.focusRing}`} type="button" aria-label={i18n("projects.moveToTrash", { title: project.title })} disabled={busy} onClick={() => setTrashOpen(true)}>
              <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false"><path d="M4 7h16M9 7V4h6v3m3 0-.8 13H6.8L6 7m4 4v5m4-5v5" /></svg>
            </button>
          )}
        </>
      )}
      <ConfirmationDialog
        open={trashOpen}
        title={i18n("projects.dialog.trashTitle")}
        description={i18n("projects.dialog.trashDescription")}
        confirmLabel={i18n("projects.moveToTrash", { title: project.title })}
        cancelLabel={i18n("projects.dialog.keepEditing")}
        busy={trashBusy}
        onConfirm={() => void confirmTrash()}
        onCancel={() => { if (!trashBusy) setTrashOpen(false) }}
      />
    </div>
  )
}
