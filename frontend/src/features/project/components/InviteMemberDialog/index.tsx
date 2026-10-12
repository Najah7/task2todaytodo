import { useEffect, useRef } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import type { InviteMemberValues } from "./schema"
import { inviteMemberSchema } from "./schema"
import { useI18n, type MessageKey } from "~/features/i18n/hooks"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

const inviteMemberResolver = zodResolver(inviteMemberSchema)

export type InviteRole = { id: string }

type Props = {
  open: boolean
  projectName: string
  roles: InviteRole[]
  busy?: boolean
  error?: MessageKey | null
  onClose: () => void
  onSubmit: (values: InviteMemberValues) => void
}

export default function InviteMemberDialog({ open, projectName, roles, busy = false, error, onClose, onSubmit }: Props) {
  const i18n = useI18n()
  const dialogRef = useRef<HTMLDialogElement>(null)
  const { register, handleSubmit, reset, formState: { errors } } = useForm<InviteMemberValues>({
    resolver: (...args) => inviteMemberResolver(...args),
    defaultValues: { email: "", roleId: "viewer" },
  })

  useEffect(() => {
    const dialog = dialogRef.current
    if (!dialog) return
    if (open && !dialog.open) dialog.showModal()
    if (!open && dialog.open) dialog.close()
  }, [open])

  useEffect(() => {
    if (open) reset({ email: "", roleId: "viewer" })
  }, [open, reset])

  return (
    <dialog
      ref={dialogRef}
      className={styles.dialog}
      aria-labelledby="invite-member-title"
      aria-busy={busy}
      onCancel={(event) => {
        event.preventDefault()
        if (!busy) onClose()
      }}
    >
      <form className={styles.form} onSubmit={handleSubmit(onSubmit)} noValidate>
        <header className={styles.header}>
          <h2 className="text-card-title" id="invite-member-title">{i18n("members.inviteDialog.title")}</h2>
          <button className={`${styles.close} ${controls.focusRing}`} type="button" aria-label={i18n("common.close")} disabled={busy} onClick={onClose}>×</button>
        </header>
        <div className={styles.fields}>
          <label className={styles.field}>
            <span className="text-label">{i18n("members.inviteDialog.email")}</span>
            <input className={`${controls.input} ${controls.focusRing}`} type="email" autoComplete="email" autoFocus aria-invalid={Boolean(errors.email)} aria-describedby={errors.email ? "invite-email-error" : "invite-email-hint"} {...register("email")} />
            {errors.email?.message && <span className={styles.error} id="invite-email-error" role="alert">{i18n(errors.email.message as MessageKey)}</span>}
            <span className="text-caption" id="invite-email-hint">{i18n("members.inviteDialog.emailHint")}</span>
          </label>
          <p className={styles.project}><span className="text-label">{i18n("members.inviteDialog.project")}</span><span className="text-body">{projectName}</span></p>
          <label className={styles.field}>
            <span className="text-label">{i18n("members.inviteDialog.role")}</span>
            <select className={`${controls.input} ${controls.focusRing}`} {...register("roleId")}>
              {roles.map((role) => <option key={role.id} value={role.id}>{i18n(`members.role.${role.id}` as MessageKey)}</option>)}
            </select>
          </label>
          {error && <p className={styles.error} role="alert">{i18n(error)}</p>}
        </div>
        <footer className={styles.actions}>
          <button className={`${controls.button} ${controls.neutralButton} ${controls.focusRing} text-body`} type="button" onClick={onClose} disabled={busy}>{i18n("members.inviteDialog.cancel")}</button>
          <button className={`${controls.button} ${controls.primaryButton} ${controls.focusRing} text-button`} type="submit" disabled={busy || roles.length === 0}>
            {busy ? i18n("members.inviteDialog.submitting") : i18n("members.inviteDialog.submit")}
          </button>
        </footer>
      </form>
    </dialog>
  )
}
