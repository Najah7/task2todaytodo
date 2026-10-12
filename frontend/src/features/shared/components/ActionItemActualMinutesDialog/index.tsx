import { useEffect, useId, useRef } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm } from "react-hook-form"
import { actionItemActualMinutesSchema, type ActionItemActualMinutesFormValues } from "./schema"
import { useI18n } from "~/features/i18n/hooks"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

type Props = {
  open: boolean
  busy?: boolean
  onComplete: (actualMinutes?: number) => void
  onCancel: () => void
}

export default function ActionItemActualMinutesDialog({ open, busy = false, onComplete, onCancel }: Props) {
  const i18n = useI18n()
  const dialogRef = useRef<HTMLDialogElement>(null)
  const id = useId()
  const form = useForm<ActionItemActualMinutesFormValues>({ resolver: zodResolver(actionItemActualMinutesSchema), defaultValues: { actualMinutes: "" } })
  const { reset } = form

  useEffect(() => {
    const dialog = dialogRef.current
    if (!dialog) return
    if (open) {
      reset({ actualMinutes: "" })
      if (typeof dialog.showModal === "function") dialog.showModal()
      else dialog.setAttribute("open", "")
    }
    return () => {
      if (typeof dialog.close === "function" && dialog.open) dialog.close()
      else dialog.removeAttribute("open")
    }
  }, [open, reset])

  if (!open) return null

  function submit(values: ActionItemActualMinutesFormValues) {
    const value = values.actualMinutes.trim()
    if (!value) {
      onComplete()
      return
    }
    onComplete(Number(value))
  }

  return <dialog
    ref={dialogRef}
    className={styles.dialog}
    aria-labelledby={`${id}-title`}
    aria-describedby={`${id}-description`}
    aria-busy={busy}
    onCancel={(event) => {
      event.preventDefault()
      if (!busy) onCancel()
    }}
  >
    <form className={styles.content} onSubmit={form.handleSubmit(submit)} noValidate>
      <h2 className="text-card-title" id={`${id}-title`}>{i18n("actualMinutes.title")}</h2>
      <p className={`${styles.description} text-body`} id={`${id}-description`}>{i18n("actualMinutes.description")}</p>
      <label className="text-field-label" htmlFor={`${id}-minutes`}>{i18n("actualMinutes.label")}</label>
      <Controller control={form.control} name="actualMinutes" render={({ field }) => <input
        className={`${controls.input} ${styles.input}`}
        id={`${id}-minutes`}
        name={field.name}
        ref={field.ref}
        value={field.value}
        onChange={field.onChange}
        onBlur={field.onBlur}
        type="number"
        min={0}
        max={2_147_483_647}
        step={1}
        inputMode="numeric"
        disabled={busy}
        aria-invalid={Boolean(form.formState.errors.actualMinutes)}
        aria-describedby={form.formState.errors.actualMinutes ? `${id}-error` : undefined}
      />} />
      {form.formState.errors.actualMinutes && <p className={styles.error} id={`${id}-error`} role="alert">{i18n("actualMinutes.invalid")}</p>}
      <div className={styles.actions}>
        <button type="button" className={`${controls.button} ${controls.neutralButton} ${controls.focusRing} text-body`} disabled={busy} onClick={onCancel}>{i18n("actualMinutes.cancel")}</button>
        <button type="button" className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} disabled={busy} onClick={() => {
          form.clearErrors("actualMinutes")
          onComplete()
        }}>{i18n("actualMinutes.skip")}</button>
        <button type="submit" className={`${controls.button} ${controls.primaryButton} ${controls.focusRing} text-button-small`} disabled={busy}>{i18n("actualMinutes.save")}</button>
      </div>
    </form>
  </dialog>
}
