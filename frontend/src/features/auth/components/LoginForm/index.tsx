import { useState } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useLogin } from "~/features/auth/hooks/useLogin"
import { loginSchema, type LoginFormValues } from "./schema"
import EmailField from "~/features/auth/components/EmailField"
import PasswordField from "~/features/auth/components/PasswordField"
import SubmitButton from "~/features/auth/components/SubmitButton"
import { useI18n, type MessageKey } from "~/features/i18n/hooks"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"
import { getAuthErrorMessage } from "~/features/auth/errors"

export default function LoginForm() {
  const i18n = useI18n()
  const { logIn } = useLogin()
  const { register, handleSubmit, formState: { errors, isSubmitting } } = useForm<LoginFormValues>({
    resolver: zodResolver(loginSchema),
    defaultValues: { email: "", password: "" },
  })
  const [showPassword, setShowPassword] = useState(false)
  const [error, setError] = useState<MessageKey | null>(null)

  async function onSubmit(data: LoginFormValues) {
    if (isSubmitting) return
    setError(null)
    try {
      await logIn(data)
    } catch (cause) {
      setError(getAuthErrorMessage(cause))
    }
  }

  return (
    <section className={`${controls.card} ${styles.card}`} aria-labelledby="login-title">
      <h1 id="login-title" className={`${styles.title} text-login-title`}>{i18n("auth.login.title")}</h1>
      <form onSubmit={handleSubmit(onSubmit, () => setError(null))} noValidate aria-busy={isSubmitting}>
        <fieldset className={styles.fields} disabled={isSubmitting}>
          <EmailField registration={register("email")} error={errors.email?.message} />
          <PasswordField registration={register("password")} error={errors.password?.message} visible={showPassword} onVisibilityChange={setShowPassword} autoComplete="current-password" />
          {error && <p className={`${styles.error} text-description`} role="alert">{i18n(error)}</p>}
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
