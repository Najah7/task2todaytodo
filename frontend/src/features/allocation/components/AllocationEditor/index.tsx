import { useEffect, useState } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useI18n, type MessageKey } from "~/features/i18n/hooks"
import type { WeeklyPlanAllocationView } from "~/features/allocation/components/WeeklyPlanBoard/types"
import ConfirmationDialog from "~/features/shared/components/ConfirmationDialog"
import { useFormDraftGuard } from "~/features/shared/hooks/useFormDraftGuard"
import { useModalFocusTrap } from "~/features/shared/hooks/useModalFocusTrap"
import controls from "~/styles/controls.module.css"
import { allocationEditorSchema, type AllocationEditorValues } from "./schema"
import styles from "./index.module.css"

type Props = {
  open: boolean
  timezone: string
  projects: Array<{ id: string; title: string }>
  initialValues: AllocationEditorValues
  allocation?: WeeklyPlanAllocationView
  busy: boolean
  onSave: (values: AllocationEditorValues) => Promise<void>
  onDelete: (scope: "current" | "following") => Promise<void>
  onCancel: () => void
}

export default function AllocationEditor({ open, timezone, projects, initialValues, allocation, busy, onSave, onDelete, onCancel }: Props) {
  const i18n = useI18n()
  const [deleteScope, setDeleteScope] = useState<"current" | "following">("current")
  const [confirmDelete, setConfirmDelete] = useState(false)
  const [discardOpen, setDiscardOpen] = useState(false)
  const { register, handleSubmit, reset, formState: { errors, isSubmitting, isDirty } } = useForm<AllocationEditorValues>({
    resolver: zodResolver(allocationEditorSchema),
    defaultValues: initialValues,
  })
  const dirtyGuard = useFormDraftGuard(isDirty)
  const cancel = () => isDirty ? setDiscardOpen(true) : onCancel()
  const modal = useModalFocusTrap<HTMLElement>(open, cancel)
  useEffect(() => { reset(initialValues) }, [initialValues, open, reset])
  const saving = busy || isSubmitting
  if (!open) return null
  const fieldError = (key?: string) => key ? i18n(key as MessageKey) : undefined
  const endError = fieldError(errors.endAt?.message)
  async function submit(values: AllocationEditorValues) {
    try {
      await onSave(values)
    } catch {
      // The action provider has already shown the request error; keep this draft open.
    }
  }
  async function submitDelete() {
    try {
      await onDelete(deleteScope)
      setConfirmDelete(false)
    } catch {
      // Keep the confirmation open so the user can retry after the action notification.
    }
  }

  return <div className={styles.overlay} role="presentation">
    <section ref={modal.ref} onKeyDown={modal.onKeyDown} className={styles.dialog} role="dialog" aria-modal="true" aria-labelledby="allocation-editor-title">
      <header className={styles.header}>
        <h2 className="text-section-title" id="allocation-editor-title">{i18n(allocation ? "weeklyPlan.editor.editTitle" : "weeklyPlan.editor.createTitle")}</h2>
        <button type="button" className={styles.close} aria-label={i18n("common.close")} onClick={cancel}>×</button>
      </header>
      <form onSubmit={handleSubmit(submit)} noValidate aria-busy={saving}>
        <fieldset disabled={saving} className={styles.fields}>
          <label className="text-field-label" htmlFor="allocation-project">{i18n("weeklyPlan.editor.project")}</label>
          <select id="allocation-project" className={controls.select} {...register("projectId")}>
            {projects.map((project) => <option key={project.id} value={project.id}>{project.title}</option>)}
          </select>
          <label className="text-field-label" htmlFor="allocation-start">{i18n("weeklyPlan.editor.start")} ({timezone})</label>
          <input id="allocation-start" className={controls.input} type="datetime-local" step={60} {...register("startAt")} />
          <label className="text-field-label" htmlFor="allocation-end">{i18n("weeklyPlan.editor.end")}</label>
          <input id="allocation-end" className={controls.input} type="datetime-local" step={60} aria-invalid={Boolean(endError)} aria-describedby={endError ? "allocation-end-error" : undefined} {...register("endAt")} />
          {endError && <p id="allocation-end-error" className={styles.error} role="alert">{endError}</p>}
          <label className={styles.checkbox}>
            <input type="checkbox" {...register("repeatWeekly")} />
            <span><strong>{i18n("weeklyPlan.editor.repeat")}</strong><small>{i18n("weeklyPlan.editor.repeatHelp")}</small></span>
          </label>
        </fieldset>
        <footer className={styles.actions}>
          {allocation?.canDelete && <button type="button" className={styles.deleteButton} disabled={saving} onClick={() => { setDeleteScope("current"); setConfirmDelete(true) }}>{i18n("weeklyPlan.editor.delete")}</button>}
          <button type="button" className={controls.button + " " + controls.neutralButton} disabled={saving} onClick={cancel}>{i18n("weeklyPlan.editor.cancel")}</button>
          <button type="submit" className={controls.button + " " + controls.primaryButton} disabled={saving}>{i18n("weeklyPlan.editor.save")}</button>
        </footer>
      </form>
      {allocation?.canDeleteFollowing && <div className={styles.deleteFollowing}><button type="button" disabled={saving} onClick={() => { setDeleteScope("following"); setConfirmDelete(true) }}>{i18n("weeklyPlan.editor.deleteFollowing")}</button></div>}
      {allocation && <ConfirmationDialog
        open={confirmDelete}
        title={i18n("weeklyPlan.editor.deleteTitle")}
        description={i18n(deleteScope === "following" ? "weeklyPlan.editor.deleteFollowingDescription" : "weeklyPlan.editor.deleteDescription")}
        confirmLabel={i18n("weeklyPlan.editor.confirmDelete")}
        cancelLabel={i18n("weeklyPlan.editor.cancel")}
        destructive
        busy={saving}
        onConfirm={() => { void submitDelete() }}
        onCancel={() => setConfirmDelete(false)}
      />}
      <ConfirmationDialog open={dirtyGuard.dialogOpen || discardOpen} title={i18n("projects.dialog.discardTitle")} description={i18n("projects.dialog.discardDescription")} confirmLabel={i18n("projects.dialog.leave")} cancelLabel={i18n("projects.dialog.keepEditing")} busy={saving} onConfirm={() => { dirtyGuard.markClean(); dirtyGuard.leave(); setDiscardOpen(false); onCancel() }} onCancel={() => { dirtyGuard.stay(); setDiscardOpen(false) }} />
    </section>
  </div>
}
