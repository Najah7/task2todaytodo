import { useState } from "react"
import { Link } from "react-router"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import EmailField from "~/features/auth/components/EmailField"
import { postPasswordResetRequest } from "~/api/generated/auth"
import { useI18n } from "~/features/i18n/hooks"
import { requestPasswordResetSchema, type RequestPasswordResetFormValues } from "./schema"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

export default function RequestPasswordResetForm() {
  const i18n = useI18n()
  const [requested, setRequested] = useState(false)
  const [error, setError] = useState(false)
  const [resendPending, setResendPending] = useState(false)
  const { register, handleSubmit, getValues, formState: { errors, isSubmitting } } = useForm<RequestPasswordResetFormValues>({
    resolver: zodResolver(requestPasswordResetSchema),
    defaultValues: { email: "" },
  })

  async function onSubmit({ email }: RequestPasswordResetFormValues) {
    if (isSubmitting) return
    setError(false)
    try {
      await postPasswordResetRequest({ email })
      setRequested(true)
    } catch {
      setError(true)
    }
  }

  async function resend() {
    if (isSubmitting || resendPending || !getValues("email")) return
    setResendPending(true)
    try {
      await postPasswordResetRequest({ email: getValues("email").trim() })
      setError(false)
    } catch {
      setError(true)
    } finally {
      setResendPending(false)
    }
  }

  return (
    <section className={`${controls.card} ${styles.card}`} aria-labelledby="reset-request-title">
      <h1 id="reset-request-title" className={`${styles.title} text-login-title`}>{i18n("auth.passwordReset.requestTitle")}</h1>
      {requested ? (
        <div className={styles.fields}>
          <p className={`${styles.note} text-description`} role="status">{i18n("auth.passwordReset.guidance")}</p>
          <button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} type="button" onClick={() => void resend()} disabled={resendPending}>{resendPending ? i18n("auth.passwordReset.requesting") : i18n("auth.passwordReset.resend")}</button>
          {error && <p className={`${styles.error} text-description`} role="alert">{i18n("auth.error.generic")}</p>}
          <Link className={`${styles.link} text-caption`} to="/login">{i18n("auth.emailVerification.loginLink")}</Link>
        </div>
      ) : (
        <form className={styles.fields} onSubmit={handleSubmit(onSubmit)} noValidate aria-busy={isSubmitting}>
          <p className={`${styles.note} text-description`}>{i18n("auth.passwordReset.requestDescription")}</p>
          <EmailField registration={register("email")} error={errors.email?.message} />
          {error && <p className={`${styles.error} text-description`} role="alert">{i18n("auth.error.generic")}</p>}
          <button className={`${controls.button} ${controls.primaryButton} ${controls.focusRing} text-button`} type="submit" disabled={isSubmitting}>{isSubmitting ? i18n("auth.passwordReset.requesting") : i18n("auth.passwordReset.requestSubmit")}</button>
        </form>
      )}
    </section>
  )
}
