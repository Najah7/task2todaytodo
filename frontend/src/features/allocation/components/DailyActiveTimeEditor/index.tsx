import { useEffect, useState } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useI18n, type MessageKey } from "~/features/i18n/hooks"
import ConfirmationDialog from "~/features/shared/components/ConfirmationDialog"
import { useModalFocusTrap } from "~/features/shared/hooks/useModalFocusTrap"
import controls from "~/styles/controls.module.css"
import type { DailyActiveTimeValues } from "./schema"
import { dailyActiveTimeSchema } from "./schema"
import styles from "./index.module.css"

type Props = {
  open: boolean
  date: string
  start: string
  end: string
  timezone: string
  revision: number
  hasException: boolean
  busy: boolean
  onSave: (values: DailyActiveTimeValues) => Promise<void>
  onReset: () => Promise<void>
  onCancel: () => void
}

export default function DailyActiveTimeEditor({ open, date, start, end, timezone, hasException, busy, onSave, onReset, onCancel }: Props) {
  const i18n = useI18n()
  const [discardOpen, setDiscardOpen] = useState(false)
  const { register, handleSubmit, reset, formState: { errors, isSubmitting, isDirty } } = useForm<DailyActiveTimeValues>({
    resolver: zodResolver(dailyActiveTimeSchema),
    defaultValues: { start, end, nextDay: end <= start },
  })
  const cancel = () => isDirty ? setDiscardOpen(true) : onCancel()
  const modal = useModalFocusTrap<HTMLElement>(open, cancel)
  useEffect(() => { reset({ start, end, nextDay: end <= start }) }, [end, open, reset, start])
  if (!open) return null
  const saving = busy || isSubmitting
  const error = errors.end?.message ? i18n(errors.end.message as MessageKey) : undefined
  async function submit(values: DailyActiveTimeValues) {
    try {
      await onSave(values)
    } catch {
      // The action provider has shown the request error; preserve the editor draft.
    }
  }
  async function resetException() {
    try {
      await onReset()
    } catch {
      // Keep the editor open after the action provider reports the reset failure.
    }
  }
  return <div className={styles.overlay} role="presentation">
    <section ref={modal.ref} onKeyDown={modal.onKeyDown} className={styles.dialog} role="dialog" aria-modal="true" aria-labelledby="active-time-editor-title">
      <header className={styles.header}><h2 id="active-time-editor-title" className="text-section-title">{i18n("weeklyPlan.activeTime.title", { date })}</h2><button type="button" aria-label={i18n("common.close")} onClick={cancel}>×</button></header>
      <form onSubmit={handleSubmit(submit)} noValidate aria-busy={saving}>
        <fieldset className={styles.fields} disabled={saving}>
          <p className="text-caption">{timezone}</p>
          <label className="text-field-label" htmlFor="active-time-start">{i18n("weeklyPlan.activeTime.start")}</label>
          <input id="active-time-start" className={controls.input} type="time" step={60} {...register("start")} />
          <label className="text-field-label" htmlFor="active-time-end">{i18n("weeklyPlan.activeTime.end")}</label>
          <input id="active-time-end" className={controls.input} type="time" step={60} aria-invalid={Boolean(error)} aria-describedby={error ? "active-time-error" : undefined} {...register("end")} />
          {error && <p className={styles.error} id="active-time-error" role="alert">{error}</p>}
          <label className={styles.checkbox}><input type="checkbox" {...register("nextDay")} /><span>{i18n("weeklyPlan.activeTime.nextDay")}</span></label>
        </fieldset>
        <footer className={styles.actions}>
          {hasException && <button type="button" className={styles.reset} disabled={saving} onClick={() => { void resetException() }}>{i18n("weeklyPlan.activeTime.reset")}</button>}
          <button type="button" className={controls.button + " " + controls.neutralButton} disabled={saving} onClick={cancel}>{i18n("weeklyPlan.activeTime.cancel")}</button>
          <button type="submit" className={controls.button + " " + controls.primaryButton} disabled={saving}>{i18n("weeklyPlan.activeTime.save")}</button>
        </footer>
      </form>
    </section>
    <ConfirmationDialog open={discardOpen} title={i18n("projects.dialog.discardTitle")} description={i18n("projects.dialog.discardDescription")} confirmLabel={i18n("projects.dialog.leave")} cancelLabel={i18n("projects.dialog.keepEditing")} busy={saving} onConfirm={onCancel} onCancel={() => setDiscardOpen(false)} />
  </div>
}
