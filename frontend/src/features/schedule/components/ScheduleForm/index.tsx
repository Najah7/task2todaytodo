import { Controller, FormProvider, type UseFormReturn } from "react-hook-form"
import type { ReactNode } from "react"
import { Link } from "react-router"
import type { MessageKey } from "~/features/i18n/messages/types"
import { useI18n } from "~/features/i18n/hooks"
import ConfirmationDialog from "~/features/shared/components/ConfirmationDialog"
import type { ScheduleFormViewProps } from "~/features/schedule/hooks/types"
import type { ScheduleFormValues } from "./schema"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

const weekdays = ["mon", "tue", "wed", "thu", "fri", "sat", "sun"] as const

export type ScheduleFormProps = {
  form: UseFormReturn<ScheduleFormValues>
  viewProps: ScheduleFormViewProps
}

export default function ScheduleForm({ form, viewProps }: ScheduleFormProps) {
  const { heading, submitLabel, onSubmit, onCancel, projectOptions, assigneeOptions, timezoneOptions, frequencyOptions,
    canAssign, canEditContent, showScopeSelector, showRecurrenceFields, recurrenceReadOnly, timezoneReadOnly,
    serverError, discardConfirmation, hasConflict, reloading, onRequestReload, reloadConfirmation } = viewProps
  const i18n = useI18n()
  const { register, control, watch, formState: { errors, isSubmitting } } = form
  const allDay = watch("allDay")
  const frequency = watch("frequency")
  const formBusy = isSubmitting || reloading
  const contentDisabled = formBusy || !canEditContent
  const fieldError = (key?: string) => key ? i18n(key as MessageKey) : undefined
  const titleError = fieldError(errors.title?.message)
  const startDateError = fieldError(errors.startDate?.message)
  const endDateError = fieldError(errors.endDate?.message)
  const startTimeError = fieldError(errors.startTime?.message)
  const endTimeError = fieldError(errors.endTime?.message)
  const weekdaysError = fieldError(errors.weekdays?.message)

  return (
    <FormProvider {...form}>
      <section className={styles.page} aria-labelledby="schedule-form-heading">
        <nav className={styles.breadcrumb} aria-label={i18n("common.breadcrumb")}>
          <Link to="/schedules" className="text-body">{i18n("page.schedules.title")}</Link>
          <span aria-hidden="true">›</span>
          <span className="text-body" aria-current="page">{i18n(heading)}</span>
        </nav>
        <header className={styles.header}>
          <div>
            <h1 className="text-page-title" id="schedule-form-heading">{i18n(heading)}</h1>
            <p className="text-description">{i18n("schedules.form.description")}</p>
          </div>
        </header>
        <form className={`${controls.card} ${styles.form}`} aria-label={i18n(heading)} onSubmit={onSubmit} noValidate aria-busy={formBusy}>
          {hasConflict && <div className={styles.conflict} role="status">
            <p className="text-body">{i18n("schedules.error.conflict")}</p>
            <button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} type="button" onClick={onRequestReload} disabled={formBusy}>{i18n("schedules.form.reloadLatest")}</button>
          </div>}
          {serverError && <p className={styles.serverError} role="alert">{i18n(serverError)}</p>}
          <fieldset className={styles.fields} disabled={formBusy}>
            <Field label="schedules.form.title" htmlFor="schedule-title" required error={titleError}>
              <input {...register("title")} className={`${controls.input} ${styles.control}`} id="schedule-title" aria-label={i18n("schedules.form.title")} autoComplete="off" required disabled={contentDisabled} aria-invalid={Boolean(titleError)} aria-describedby={titleError ? "schedule-title-error" : undefined} />
            </Field>
            <div className={`${styles.field} ${styles.wide}`}>
              <span className={`${styles.fieldLabel} text-field-label`}>{i18n("schedules.form.date")}</span>
              <div className={styles.fieldContent}>
                <div className={styles.dateRange}>
                  <div className={styles.datePair}>
                    <label className="text-detail" htmlFor="schedule-start-date">{i18n("schedules.form.startDate")}</label>
                    <input {...register("startDate")} className={`${controls.input} ${styles.dateInput}`} id="schedule-start-date" type="date" disabled={contentDisabled} aria-invalid={Boolean(startDateError)} aria-describedby={startDateError ? "schedule-start-date-error" : undefined} />
                  </div>
                  <div className={styles.datePair}>
                    <label className="text-detail" htmlFor="schedule-end-date">{i18n("schedules.form.endDate")}</label>
                    <input {...register("endDate")} className={`${controls.input} ${styles.dateInput}`} id="schedule-end-date" type="date" disabled={contentDisabled} aria-invalid={Boolean(endDateError)} aria-describedby={endDateError ? "schedule-end-date-error" : undefined} />
                  </div>
                </div>
                {(startDateError || endDateError) && <p className={styles.error} id={startDateError ? "schedule-start-date-error" : "schedule-end-date-error"}>{startDateError ?? endDateError}</p>}
              </div>
            </div>
            <div className={styles.field}>
              <span className={`${styles.fieldLabel} text-field-label`}>{i18n("schedules.form.time")}</span>
              <div className={styles.fieldContent}>
                <div className={styles.timeRow}>
                  <label className="text-detail" htmlFor="schedule-start-time">{i18n("schedules.form.startTime")}</label>
                  <input {...register("startTime")} className={`${controls.input} ${styles.timeInput}`} id="schedule-start-time" type="time" disabled={allDay || contentDisabled} aria-invalid={Boolean(startTimeError)} aria-describedby={startTimeError ? "schedule-start-time-error" : undefined} />
                  <span aria-hidden="true">–</span>
                  <label className="text-detail" htmlFor="schedule-end-time">{i18n("schedules.form.endTime")}</label>
                  <input {...register("endTime")} className={`${controls.input} ${styles.timeInput}`} id="schedule-end-time" type="time" disabled={allDay || contentDisabled} aria-invalid={Boolean(endTimeError)} aria-describedby={endTimeError ? "schedule-end-time-error" : undefined} />
                  <label className={`${styles.inlineCheck} text-detail`}>
                    <input {...register("allDay")} type="checkbox" disabled={contentDisabled} />
                    {i18n("schedules.form.allDay")}
                  </label>
                </div>
                {(startTimeError || endTimeError) && <p className={styles.error} id={startTimeError ? "schedule-start-time-error" : "schedule-end-time-error"}>{startTimeError ?? endTimeError}</p>}
              </div>
            </div>
            <Field label="schedules.form.timezone" htmlFor="schedule-timezone" error={fieldError(errors.timezone?.message)}>
              <select {...register("timezone")} className={`${controls.select} ${controls.focusRing} ${styles.control}`} id="schedule-timezone" disabled={timezoneReadOnly || contentDisabled} aria-invalid={Boolean(errors.timezone)} aria-describedby={errors.timezone ? "schedule-timezone-error" : undefined}>
                {timezoneOptions.map((timezone) => <option key={timezone} value={timezone}>{timezone}</option>)}
              </select>
            </Field>
            <Field label="schedules.form.frequency" htmlFor="schedule-frequency" error={fieldError(errors.frequency?.message)}>
              <select {...register("frequency")} className={`${controls.select} ${controls.focusRing} ${styles.control}`} id="schedule-frequency" disabled={recurrenceReadOnly || contentDisabled} aria-invalid={Boolean(errors.frequency)} aria-describedby={errors.frequency ? "schedule-frequency-error" : undefined}>
                {frequencyOptions.map((value) => <option key={value} value={value}>{i18n(`schedules.form.frequency.${value}`)}</option>)}
              </select>
            </Field>
            {showRecurrenceFields && frequency !== "once" && <>
              <Field label="schedules.form.interval" htmlFor="schedule-interval" error={fieldError(errors.interval?.message)}>
                <div className={styles.intervalRow}><span className="text-body">{i18n(`schedules.form.frequency.${frequency}`)}</span><input {...register("interval", { valueAsNumber: true })} className={`${controls.input} ${styles.intervalInput}`} id="schedule-interval" type="number" min="1" step="1" disabled={recurrenceReadOnly || contentDisabled} aria-invalid={Boolean(errors.interval)} /><span className="text-body">{i18n("schedules.form.intervalSuffix")}</span></div>
              </Field>
              {frequency === "weekly" && <Field label="schedules.form.weekdays" htmlFor="schedule-weekday-mon" error={weekdaysError}>
                <Controller control={control} name="weekdays" render={({ field }) => <div className={styles.weekdays} role="group" aria-label={i18n("schedules.form.weekdays")}>
                  {weekdays.map((day) => <label key={day} className={`${styles.weekday} text-detail`}><input id={`schedule-weekday-${day}`} type="checkbox" value={day} disabled={recurrenceReadOnly || contentDisabled} checked={field.value.includes(day)} onChange={(event) => field.onChange(event.target.checked ? [...field.value, day] : field.value.filter((value) => value !== day))} />{i18n(`schedules.form.weekday.${day}`)}</label>)}
                </div>} />
                {weekdaysError && <p className={styles.error}>{weekdaysError}</p>}
              </Field>}
              <Field label="schedules.form.recurrenceEndDate" htmlFor="schedule-recurrence-end" error={fieldError(errors.recurrenceEndDate?.message)}>
                <input {...register("recurrenceEndDate")} className={`${controls.input} ${styles.dateInput}`} id="schedule-recurrence-end" type="date" disabled={recurrenceReadOnly || contentDisabled} aria-invalid={Boolean(errors.recurrenceEndDate)} />
              </Field>
            </>}
            <Field label="schedules.form.project" htmlFor="schedule-project" error={fieldError(errors.projectId?.message)}>
              <select {...register("projectId", { onChange: (event) => viewProps.onProjectChange(event.target.value) })} className={`${controls.select} ${controls.focusRing} ${styles.control}`} id="schedule-project" disabled={contentDisabled} aria-invalid={Boolean(errors.projectId)} aria-describedby={errors.projectId ? "schedule-project-error" : undefined}>
                <option value="">{i18n("schedules.form.noProject")}</option>
                {projectOptions.map((project) => <option key={project.id} value={project.id}>{project.title}</option>)}
              </select>
            </Field>
            <Field label="schedules.form.assignee" htmlFor="schedule-assignee" error={fieldError(errors.assigneeId?.message)}>
              <select {...register("assigneeId")} className={`${controls.select} ${controls.focusRing} ${styles.control}`} id="schedule-assignee" disabled={!canAssign || formBusy || assigneeOptions.length === 0} aria-invalid={Boolean(errors.assigneeId)} aria-describedby={errors.assigneeId ? "schedule-assignee-error" : !canAssign ? "schedule-assignee-help" : undefined}>
                <option value="">{i18n("schedules.form.autoAssignee")}</option>
                {assigneeOptions.map((person) => <option key={person.id} value={person.id}>{person.displayName}</option>)}
              </select>
              {!canAssign && <p className="text-caption" id="schedule-assignee-help">{i18n("schedules.form.assigneePermission")}</p>}
            </Field>
            <Field label="schedules.form.location" htmlFor="schedule-location">
              <input {...register("location")} className={`${controls.input} ${styles.control}`} id="schedule-location" autoComplete="off" disabled={contentDisabled} />
            </Field>
            <Field label="schedules.form.notes" htmlFor="schedule-description">
              <textarea {...register("description")} className={`${controls.input} ${styles.control} ${styles.textarea}`} id="schedule-description" rows={4} disabled={contentDisabled} />
            </Field>
            {showScopeSelector && <Field label="schedules.form.scope" htmlFor="schedule-scope" error={fieldError(errors.scope?.message)}>
              <select {...register("scope")} className={`${controls.select} ${controls.focusRing} ${styles.control}`} id="schedule-scope" disabled={formBusy} aria-invalid={Boolean(errors.scope)}>
                <option value="current">{i18n("schedules.form.scopeCurrent")}</option>
                <option value="following">{i18n("schedules.form.scopeFollowing")}</option>
              </select>
            </Field>}
            <input type="hidden" {...register("recurrenceAnchorDate")} />
          </fieldset>
          <footer className={styles.actions}>
            <button className={`${controls.button} ${controls.neutralButton} ${controls.focusRing} text-body`} type="button" disabled={formBusy} onClick={onCancel}>{i18n("schedules.form.cancel")}</button>
            <button className={`${controls.button} ${controls.primaryButton} ${controls.focusRing} text-button`} type="submit" disabled={formBusy}>{i18n(formBusy ? "schedules.form.submitting" : submitLabel)}</button>
          </footer>
        </form>
        <ConfirmationDialog open={discardConfirmation.open} title={i18n("schedules.dialog.discardTitle")} description={i18n("schedules.dialog.discardDescription")} confirmLabel={i18n("schedules.dialog.discard")} cancelLabel={i18n("schedules.dialog.keepEditing")} busy={formBusy} onConfirm={discardConfirmation.onConfirm} onCancel={discardConfirmation.onCancel} />
        {hasConflict && <ConfirmationDialog open={reloadConfirmation.open} title={i18n("schedules.dialog.conflictTitle")} description={i18n("schedules.dialog.conflictDescription")} confirmLabel={i18n("schedules.form.reloadLatest")} cancelLabel={i18n("schedules.dialog.keepEditing")} busy={reloading} onConfirm={reloadConfirmation.onConfirm} onCancel={reloadConfirmation.onCancel} />}
      </section>
    </FormProvider>
  )
}

function Field({ label, htmlFor, children, required, error }: { label: MessageKey; htmlFor: string; children: ReactNode; required?: boolean; error?: string }) {
  const i18n = useI18n()
  const errorId = `${htmlFor}-error`
  return <div className={styles.field}>
    <label className={`${styles.fieldLabel} text-field-label`} htmlFor={htmlFor}>{i18n(label)}{required && <span className={styles.required} aria-hidden="true">{i18n("schedules.form.titleRequired")}</span>}</label>
    <div className={styles.fieldContent}>
      {children}
      {error && <p className={styles.error} id={errorId}>{error}</p>}
    </div>
  </div>
}
