import { useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm } from "react-hook-form"
import { Link, useNavigate } from "react-router"
import { useI18n, type MessageKey } from "~/features/i18n/hooks"
import ConfirmationDialog from "~/features/project/components/ConfirmationDialog"
import { useProjectDraftGuard } from "~/features/project/hooks/useProjectDraftGuard"
import { notify } from "~/features/shared/notification"
import SeachSelect from "~/features/shared/components/SeachSelect"
import controls from "~/styles/controls.module.css"
import { emptyProjectFormValues, projectFormSchema, type ProjectFormValues } from "./schema"
import styles from "./index.module.css"

export type ProjectFormOption = { value: string; label: string }
export type ProjectSubmitResult =
  | { saved: true; revision: number }
  | { saved: false; fieldErrors?: Partial<Record<keyof ProjectFormValues, MessageKey>> }

export type ProjectFormReloadSnapshot = { values: ProjectFormValues; revision: number }

type Props = {
  mode: "create" | "edit"
  heading: MessageKey
  initialValues?: ProjectFormValues
  types: ProjectFormOption[]
  priorities: ProjectFormOption[]
  submitting?: boolean
  hasConflict?: boolean
  onSubmit: (values: ProjectFormValues) => Promise<ProjectSubmitResult>
  onSaveComplete?: (revision: number) => void
  onLoadLatest?: () => Promise<ProjectFormReloadSnapshot>
  onConflictResolved?: (revision: number) => void
  onError?: (error: unknown) => void
}

export default function ProjectForm({
  mode,
  heading,
  initialValues = emptyProjectFormValues,
  types,
  priorities,
  submitting = false,
  hasConflict = false,
  onSubmit,
  onSaveComplete,
  onLoadLatest,
  onConflictResolved,
  onError,
}: Props) {
  const i18n = useI18n()
  const navigate = useNavigate()
  const [confirmReload, setConfirmReload] = useState(false)
  const [loadingLatest, setLoadingLatest] = useState(false)
  const { control, register, handleSubmit, reset, setError, formState: { errors, isDirty, isSubmitting } } = useForm<ProjectFormValues>({
    resolver: zodResolver(projectFormSchema),
    defaultValues: initialValues,
  })
  const dirtyGuard = useProjectDraftGuard(isDirty)
  const busy = submitting || isSubmitting || loadingLatest

  async function submit(values: ProjectFormValues) {
    try {
      const result = await onSubmit(values)
      if (!result.saved) {
        for (const [field, message] of Object.entries(result.fieldErrors ?? {})) {
          setError(field as keyof ProjectFormValues, { type: "server", message })
        }
        return
      }
      dirtyGuard.markClean()
      reset(values)
      onSaveComplete?.(result.revision)
    } catch (cause) {
      if (onError) onError(cause)
      else notify.error(i18n("projects.error.generic"))
    }
  }

  async function reloadLatest() {
    if (!onLoadLatest) return
    setLoadingLatest(true)
    try {
      const latest = await onLoadLatest()
      reset(latest.values)
      onConflictResolved?.(latest.revision)
      setConfirmReload(false)
    } catch (cause) {
      if (onError) onError(cause)
      else notify.error(i18n("projects.error.generic"))
    } finally {
      setLoadingLatest(false)
    }
  }

  function fieldError(key?: string) {
    return key ? i18n(key as MessageKey) : null
  }

  const titleError = fieldError(errors.title?.message)
  const typeError = fieldError(errors.type?.message)
  const priorityError = fieldError(errors.priority?.message)
  const startDateError = fieldError(errors.startDate?.message)
  const endDateError = fieldError(errors.endDate?.message)

  return (
    <section className={styles.page} aria-labelledby="project-form-heading">
      <nav className={styles.breadcrumb} aria-label={i18n("common.breadcrumb")}>
        <Link to="/projects" className="text-body">{i18n("page.projects.title")}</Link>
        <span aria-hidden="true">›</span>
        <span className="text-body" aria-current="page">{i18n(heading)}</span>
      </nav>
      <header className={styles.header}>
        <div>
          <h1 className="text-page-title" id="project-form-heading">{i18n(heading)}</h1>
          <p className="text-description">{i18n("projects.form.description")}</p>
        </div>
      </header>
      <form className={`${controls.card} ${styles.form}`} onSubmit={handleSubmit(submit)} noValidate aria-busy={busy}>
        {hasConflict && (
          <div className={styles.conflict} role="status">
            <p className="text-body">{i18n("projects.error.conflict")}</p>
            <button
              className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`}
              type="button"
              onClick={() => setConfirmReload(true)}
              disabled={busy}
            >
              {i18n("projects.dialog.reloadLatest")}
            </button>
          </div>
        )}
        <fieldset className={styles.fields} disabled={busy}>
          <div className={`${styles.field} ${styles.wide}`}>
            <label className={`${styles.fieldLabel} text-field-label`} htmlFor="project-title">
              {i18n("projects.form.title")} <span className={styles.required}>{i18n("projects.form.titleRequired")}</span>
            </label>
            <div className={styles.fieldContent}>
              <input
                {...register("title")}
                className={`${controls.input} ${styles.control}`}
                id="project-title"
                autoComplete="off"
                placeholder={i18n("projects.form.titlePlaceholder")}
                required
                aria-invalid={Boolean(titleError)}
                aria-describedby={titleError ? "project-title-error" : undefined}
              />
              {titleError && <p className={styles.error} id="project-title-error">{titleError}</p>}
            </div>
          </div>
          <div className={`${styles.field} ${styles.wide}`}>
            <label className={`${styles.fieldLabel} text-field-label`} htmlFor="project-goal">{i18n("projects.form.goal")}</label>
            <div className={styles.fieldContent}>
              <input
                {...register("goal")}
                className={`${controls.input} ${styles.control}`}
                id="project-goal"
                placeholder={i18n("projects.form.goalHint")}
              />
              <p className="text-caption">{i18n("projects.form.goalHelp")}</p>
            </div>
          </div>
          <div className={`${styles.field} ${styles.wide}`}>
            <label className={`${styles.fieldLabel} text-field-label`} htmlFor="project-description">{i18n("projects.form.descriptionLabel")}</label>
            <div className={styles.fieldContent}>
              <textarea
                {...register("description")}
                className={`${controls.input} ${styles.control} ${styles.textarea}`}
                id="project-description"
                placeholder={i18n("projects.form.descriptionPlaceholder")}
                rows={5}
              />
            </div>
          </div>
          <div className={`${styles.field} ${styles.wide}`}>
            <label id="project-type-label" className={`${styles.fieldLabel} text-field-label`} htmlFor="project-type">{i18n("projects.form.type")}</label>
            <div className={styles.fieldContent}>
              <Controller control={control} name="type" render={({ field }) => (
                <SeachSelect
                  ref={field.ref}
                  id="project-type"
                  labelledBy="project-type-label"
                  className={styles.control}
                  value={field.value}
                  options={types}
                  onChange={field.onChange}
                  onBlur={field.onBlur}
                  disabled={busy}
                  invalid={Boolean(typeError)}
                  describedBy={typeError ? "project-type-error" : undefined}
                />
              )} />
              {typeError && <p className={styles.error} id="project-type-error">{typeError}</p>}
            </div>
          </div>
          <div className={`${styles.field} ${styles.wide}`}>
            <label id="project-priority-label" className={`${styles.fieldLabel} text-field-label`} htmlFor="project-priority">{i18n("projects.form.priority")}</label>
            <div className={styles.fieldContent}>
              <Controller control={control} name="priority" render={({ field }) => (
                <SeachSelect
                  ref={field.ref}
                  id="project-priority"
                  labelledBy="project-priority-label"
                  className={styles.control}
                  value={field.value}
                  options={priorities}
                  onChange={field.onChange}
                  onBlur={field.onBlur}
                  disabled={busy}
                  invalid={Boolean(priorityError)}
                  describedBy={priorityError ? "project-priority-error" : undefined}
                />
              )} />
              {priorityError && <p className={styles.error} id="project-priority-error">{priorityError}</p>}
            </div>
          </div>
          <div className={`${styles.field} ${styles.wide}`}>
            <label className={`${styles.fieldLabel} text-field-label`} htmlFor="project-start-date">{i18n("projects.form.startDate")}</label>
            <div className={styles.fieldContent}>
              <input
                {...register("startDate")}
                className={`${controls.input} ${styles.control}`}
                id="project-start-date"
                type="date"
                aria-invalid={Boolean(startDateError)}
                aria-describedby={startDateError ? "project-start-date-error" : undefined}
              />
              {startDateError && <p className={styles.error} id="project-start-date-error">{startDateError}</p>}
            </div>
          </div>
          <div className={`${styles.field} ${styles.wide}`}>
            <label className={`${styles.fieldLabel} text-field-label`} htmlFor="project-end-date">{i18n("projects.form.endDate")}</label>
            <div className={styles.fieldContent}>
              <input
                {...register("endDate")}
                className={`${controls.input} ${styles.control}`}
                id="project-end-date"
                type="date"
                aria-invalid={Boolean(endDateError)}
                aria-describedby={endDateError ? "project-end-date-error" : undefined}
              />
              {endDateError && <p className={styles.error} id="project-end-date-error">{endDateError}</p>}
            </div>
          </div>
        </fieldset>
        <footer className={styles.actions}>
          <button
            className={`${controls.button} ${controls.neutralButton} ${controls.focusRing} text-body`}
            type="button"
            disabled={busy}
            onClick={() => navigate("/projects")}
          >
            {i18n("projects.form.cancel")}
          </button>
          <button
            className={`${controls.button} ${controls.primaryButton} ${controls.focusRing} text-button`}
            type="submit"
            disabled={busy}
          >
            {busy ? i18n("projects.form.submitting") : i18n(mode === "edit" ? "projects.form.saveSubmit" : "projects.form.createSubmit")}
          </button>
        </footer>
      </form>
      <ConfirmationDialog
        open={dirtyGuard.dialogOpen}
        title={i18n("projects.dialog.discardTitle")}
        description={i18n("projects.dialog.discardDescription")}
        confirmLabel={i18n("projects.dialog.leave")}
        cancelLabel={i18n("projects.dialog.keepEditing")}
        busy={busy}
        onConfirm={dirtyGuard.leave}
        onCancel={dirtyGuard.stay}
      />
      <ConfirmationDialog
        open={confirmReload}
        title={i18n("projects.dialog.conflictTitle")}
        description={i18n("projects.dialog.conflictDescription")}
        confirmLabel={i18n("projects.dialog.reloadLatest")}
        cancelLabel={i18n("projects.dialog.keepEditing")}
        busy={loadingLatest}
        onConfirm={reloadLatest}
        onCancel={() => setConfirmReload(false)}
      />
    </section>
  )
}
