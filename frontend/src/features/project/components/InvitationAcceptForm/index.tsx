import { useState } from "react"
import { Link } from "react-router"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import PasswordField from "~/features/auth/components/PasswordField"
import SubmitButton from "~/features/auth/components/SubmitButton"
import { useI18n, type MessageKey } from "~/features/i18n/hooks"
import controls from "~/styles/controls.module.css"
import type { InvitationLoginValues, InvitationNewAccountValues } from "./schema"
import { invitationLoginSchema, invitationNewAccountSchema } from "./schema"
import styles from "./index.module.css"

type Props = {
  email: string
  mode: "new" | "unverified" | "verified"
  signedIn?: boolean
  accountMismatch?: boolean
  busy?: boolean
  error?: MessageKey | null
  tokenHash?: string
  onAcceptWithPassword: (password: string) => void
  onLogin: (password: string) => void
  onAcceptSignedIn: () => void
  onSwitchAccount: () => void
}

export default function InvitationAcceptForm({ email, mode, signedIn = false, accountMismatch = false, busy = false, error, tokenHash = "", onAcceptWithPassword, onLogin, onAcceptSignedIn, onSwitchAccount }: Props) {
  const i18n = useI18n()
  const [visible, setVisible] = useState(false)
  const loginForm = useForm<InvitationLoginValues>({
    resolver: zodResolver(invitationLoginSchema),
    defaultValues: { password: "" },
  })
  const newAccountForm = useForm<InvitationNewAccountValues>({
    resolver: zodResolver(invitationNewAccountSchema),
    defaultValues: { password: "", passwordConfirmation: "" },
  })

  const isNewAccount = mode !== "verified"
  const descriptionKey = mode === "new"
    ? "members.invitation.newAccount"
    : mode === "unverified"
      ? "members.invitation.unverifiedAccount"
      : "members.invitation.loginRequired"

  return (
    <div className={styles.form}>
      <p className={`${styles.email} text-body`}><strong>{i18n("members.invitation.email", { email })}</strong></p>
      <p className={`${styles.description} text-description`}>{i18n(descriptionKey)}</p>
      {accountMismatch ? (
        <div className={styles.actions}>
          <p className={styles.error} role="alert">{i18n("members.invitation.accountMismatch")}</p>
          <Link className={`${styles.linkButton} ${controls.focusRing} text-caption`} to={`/login${window.location.search}${tokenHash}`} onClick={onSwitchAccount}>{i18n("members.invitation.switchAccount")}</Link>
        </div>
      ) : mode === "verified" && signedIn ? (
        <div className={styles.actions}>
          <button className={`${controls.button} ${controls.primaryButton} ${controls.focusRing} text-button`} type="button" disabled={busy} onClick={onAcceptSignedIn}>
            {busy ? i18n("members.invitation.accepting") : i18n("members.invitation.accept")}
          </button>
          <button className={`${styles.linkButton} ${controls.focusRing} text-caption`} type="button" disabled={busy} onClick={onSwitchAccount}>{i18n("members.invitation.switchAccount")}</button>
        </div>
      ) : isNewAccount ? (
        <form className={styles.fields} onSubmit={newAccountForm.handleSubmit((values) => onAcceptWithPassword(values.password))} noValidate aria-busy={busy}>
          <PasswordField registration={newAccountForm.register("password")} error={newAccountForm.formState.errors.password?.message} visible={visible} autoComplete="new-password" help={i18n("auth.password.help")} />
          <PasswordField id="invitation-password-confirmation" registration={newAccountForm.register("passwordConfirmation")} error={newAccountForm.formState.errors.passwordConfirmation?.message} label={i18n("auth.password.confirmation")} visible={visible} onVisibilityChange={setVisible} autoComplete="new-password" />
          {error && <p className={styles.error} role="alert">{i18n(error)}</p>}
          <SubmitButton submitting={busy} pendingLabel={i18n("members.invitation.accepting")}>{i18n("members.invitation.accept")}</SubmitButton>
        </form>
      ) : (
        <form className={styles.fields} onSubmit={loginForm.handleSubmit((values) => onLogin(values.password))} noValidate aria-busy={busy}>
          <PasswordField registration={loginForm.register("password")} error={loginForm.formState.errors.password?.message} visible={visible} onVisibilityChange={setVisible} autoComplete="current-password" />
          {error && <p className={styles.error} role="alert">{i18n(error)}</p>}
          <SubmitButton submitting={busy} pendingLabel={i18n("members.invitation.accepting")}>{i18n("members.invitation.login")}</SubmitButton>
          <Link className={`${styles.linkButton} ${controls.focusRing} text-caption`} to={`/login${window.location.search}${tokenHash}`} onClick={onSwitchAccount}>{i18n("members.invitation.switchAccount")}</Link>
        </form>
      )}
      {mode === "verified" && signedIn && error && <p className={styles.error} role="alert">{i18n(error)}</p>}
    </div>
  )
}
