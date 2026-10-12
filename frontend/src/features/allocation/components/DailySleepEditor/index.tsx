import { useEffect, useState } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useI18n, type MessageKey } from "~/features/i18n/hooks"
import ConfirmationDialog from "~/features/shared/components/ConfirmationDialog"
import { useModalFocusTrap } from "~/features/shared/hooks/useModalFocusTrap"
import controls from "~/styles/controls.module.css"
import type { DailySleepValues } from "./schema"
import { dailySleepSchema } from "./schema"
import styles from "./index.module.css"

type Props = {
  open: boolean
  date: string
  wake: string
  bedtime: string
  timezone: string
  revision: number
  hasException: boolean
  busy: boolean
  onSave: (values: DailySleepValues) => Promise<void>
  onReset: () => Promise<void>
  onCancel: () => void
}

export default function DailySleepEditor({ open, date, wake, bedtime, timezone, hasException, busy, onSave, onReset, onCancel }: Props) {
  const i18n = useI18n()
  const [discardOpen, setDiscardOpen] = useState(false)
  const { register, handleSubmit, reset, formState: { errors, isSubmitting, isDirty } } = useForm<DailySleepValues>({
    resolver: zodResolver(dailySleepSchema),
    defaultValues: { wake, bedtime, nextDay: bedtime <= wake },
  })
  const cancel = () => isDirty ? setDiscardOpen(true) : onCancel()
  const modal = useModalFocusTrap<HTMLElement>(open, cancel)
  useEffect(() => { reset({ wake, bedtime, nextDay: bedtime <= wake }) }, [bedtime, open, reset, wake])
  if (!open) return null
  const saving = busy || isSubmitting
  const error = errors.bedtime?.message ? i18n(errors.bedtime.message as MessageKey) : undefined
  async function submit(values: DailySleepValues) {
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
    <section ref={modal.ref} onKeyDown={modal.onKeyDown} className={styles.dialog} role="dialog" aria-modal="true" aria-labelledby="sleep-editor-title">
      <header className={styles.header}><h2 id="sleep-editor-title" className="text-section-title">{i18n("weeklyPlan.sleep.title", { date })}</h2><button type="button" aria-label={i18n("common.close")} onClick={cancel}>×</button></header>
      <form onSubmit={handleSubmit(submit)} noValidate aria-busy={saving}>
        <fieldset className={styles.fields} disabled={saving}>
          <p className="text-caption">{timezone}</p>
          <label className="text-field-label" htmlFor="sleep-wake">{i18n("weeklyPlan.sleep.wake")}</label>
          <input id="sleep-wake" className={controls.input} type="time" step={60} {...register("wake")} />
          <label className="text-field-label" htmlFor="sleep-bedtime">{i18n("weeklyPlan.sleep.bedtime")}</label>
          <input id="sleep-bedtime" className={controls.input} type="time" step={60} aria-invalid={Boolean(error)} aria-describedby={error ? "sleep-error" : undefined} {...register("bedtime")} />
          {error && <p className={styles.error} id="sleep-error" role="alert">{error}</p>}
          <label className={styles.checkbox}><input type="checkbox" {...register("nextDay")} /><span>{i18n("weeklyPlan.sleep.nextDay")}</span></label>
        </fieldset>
        <footer className={styles.actions}>
          {hasException && <button type="button" className={styles.reset} disabled={saving} onClick={() => { void resetException() }}>{i18n("weeklyPlan.sleep.reset")}</button>}
          <button type="button" className={controls.button + " " + controls.neutralButton} disabled={saving} onClick={cancel}>{i18n("weeklyPlan.sleep.cancel")}</button>
          <button type="submit" className={controls.button + " " + controls.primaryButton} disabled={saving}>{i18n("weeklyPlan.sleep.save")}</button>
        </footer>
      </form>
    </section>
    <ConfirmationDialog open={discardOpen} title={i18n("projects.dialog.discardTitle")} description={i18n("projects.dialog.discardDescription")} confirmLabel={i18n("projects.dialog.leave")} cancelLabel={i18n("projects.dialog.keepEditing")} busy={saving} onConfirm={onCancel} onCancel={() => setDiscardOpen(false)} />
  </div>
}
