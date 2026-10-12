import { useState } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useLogin } from "~/features/auth/hooks/useLogin"
import { loginSchema, type LoginFormValues } from "./schema"
import EmailField from "~/features/auth/components/EmailField"
import PasswordField from "~/features/auth/components/PasswordField"
import SubmitButton from "~/features/auth/components/SubmitButton"
import { useI18n } from "~/features/i18n/hooks"
import { notify } from "~/features/shared/notification"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"
import { getAuthErrorMessage } from "~/features/auth/errors"
import { ApiError } from "~/api/http"
import { postEmailVerificationResend } from "~/api/generated/auth"
import { Link } from "react-router"
import { useLocation } from "react-router"
import type { RestErrResponse } from "~/api/generated/auth"

export default function LoginForm() {
  const i18n = useI18n()
  const location = useLocation()
  const { logIn } = useLogin()
  const { register, handleSubmit, getValues, formState: { errors, isSubmitting } } = useForm<LoginFormValues>({
    resolver: zodResolver(loginSchema),
    defaultValues: { email: "", password: "" },
  })
  const [showPassword, setShowPassword] = useState(false)
  const [emailUnverified, setEmailUnverified] = useState(false)
  const [resendComplete, setResendComplete] = useState(false)
  const [resendPending, setResendPending] = useState(false)
  const [resendError, setResendError] = useState(false)

  async function onSubmit(data: LoginFormValues) {
    if (isSubmitting) return
    setEmailUnverified(false)
    setResendComplete(false)
    try {
      await logIn(data)
    } catch (cause) {
      if (cause instanceof ApiError) {
        const details = (cause.data as RestErrResponse | undefined)?.error?.details ?? []
        if (cause.status === 403 && details.some((detail) => detail.code === "email_unverified")) {
          setEmailUnverified(true)
          return
        }
      }
      notify.error(i18n(getAuthErrorMessage(cause)))
    }
  }

  async function resendVerification() {
    if (resendPending) return
    const email = getValues("email").trim()
    if (!email) return
    setResendPending(true)
    setResendError(false)
    try {
      await postEmailVerificationResend({ email })
      setResendComplete(true)
    } catch {
      setResendError(true)
    } finally {
      setResendPending(false)
    }
  }

  return (
    <section className={`${controls.card} ${styles.card}`} aria-labelledby="login-title">
      <h1 id="login-title" className={`${styles.title} text-login-title`}>{i18n("auth.login.title")}</h1>
      {new URLSearchParams(location.search).has("verified") && <p className={`${styles.note} text-description`} role="status">{i18n("auth.login.emailVerified")}</p>}
      {new URLSearchParams(location.search).has("password-reset") && <p className={`${styles.note} text-description`} role="status">{i18n("auth.login.passwordReset")}</p>}
      <form onSubmit={handleSubmit(onSubmit)} noValidate aria-busy={isSubmitting}>
        <fieldset className={styles.fields} disabled={isSubmitting}>
          <EmailField registration={register("email")} error={errors.email?.message} />
          <PasswordField registration={register("password")} error={errors.password?.message} visible={showPassword} onVisibilityChange={setShowPassword} autoComplete="current-password" />
          <Link className={`${styles.forgotPassword} text-caption`} to="/request-password-reset">{i18n("auth.passwordReset.forgot")}</Link>
          {emailUnverified && (
            <div className={styles.unverified}>
              <p className={`${styles.note} text-description`} role="alert">{i18n("auth.emailVerification.loginRequired")}</p>
              {resendComplete ? <p className={`${styles.note} text-description`} role="status">{i18n("auth.emailVerification.guidance")}</p> : (
                <button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} type="button" onClick={() => void resendVerification()} disabled={resendPending}>
                  {resendPending ? i18n("auth.emailVerification.resending") : i18n("auth.emailVerification.resend")}
                </button>
              )}
              {resendError && <p className={`${styles.error} text-description`} role="alert">{i18n("auth.error.generic")}</p>}
            </div>
          )}
          <SubmitButton submitting={isSubmitting} pendingLabel={i18n("auth.login.submitting")}>{i18n("auth.login.submit")}</SubmitButton>
        </fieldset>
      </form>
      <div className={styles.social}>
        <div className={`${styles.divider} text-caption`}>
          <span className={styles.dividerLine} aria-hidden="true" />{i18n("auth.login.or")}<span className={styles.dividerLine} aria-hidden="true" />
        </div>
        <p className={`${styles.note} text-caption`}>{i18n("auth.login.socialUnavailable")}</p>
        {["Google", "X", "GitHub"].map((provider) => (
          <button key={provider} className={`${controls.button} ${controls.neutralButton} text-body`} type="button" disabled>{i18n("auth.login.socialProvider", { provider })}</button>
        ))}
      </div>
    </section>
  )
}
