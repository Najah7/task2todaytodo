import { useState } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { usePostSignup } from "~/api/generated/auth"
import { signupSchema, type SignupFormValues } from "./schema"
import { useLogin } from "~/features/auth/hooks/useLogin"
import { getAuthErrorMessage } from "~/features/auth/errors"
import EmailField from "~/features/auth/components/EmailField"
import PasswordField from "~/features/auth/components/PasswordField"
import SubmitButton from "~/features/auth/components/SubmitButton"
import { useI18n, type MessageKey } from "~/features/i18n/hooks"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

export default function SignupForm() {
  const i18n = useI18n()
  const signup = usePostSignup({ mutation: { retry: false, gcTime: 0 } })
  const { logIn } = useLogin()
  const { register, handleSubmit, formState: { errors, isSubmitting } } = useForm<SignupFormValues>({
    resolver: zodResolver(signupSchema),
    defaultValues: { email: "", password: "", passwordConfirmation: "" },
  })
  const [showPassword, setShowPassword] = useState(false)
  const [accountCreated, setAccountCreated] = useState(false)
  const [error, setError] = useState<MessageKey | null>(null)

  async function onSubmit({ email, password }: SignupFormValues) {
    if (isSubmitting) return
    setError(null)
    const data = { email, password }
    let created = accountCreated
    try {
      if (!created) {
        await signup.mutateAsync({ data })
        created = true
        setAccountCreated(true)
      }
      await logIn(data)
    } catch (cause) {
      setError(getAuthErrorMessage(cause))
    }
  }

  const actionLabel = accountCreated ? i18n("auth.signup.retryLogin") : i18n("auth.signup.submit")

  return (
    <section className={`${controls.card} ${styles.card}`} aria-labelledby="signup-title">
      <h1 id="signup-title" className={`${styles.title} text-login-title`}>{i18n("auth.signup.title")}</h1>
      <form onSubmit={handleSubmit(onSubmit, () => setError(null))} noValidate aria-busy={isSubmitting}>
        <fieldset className={styles.fields} disabled={isSubmitting}>
          <EmailField registration={register("email")} error={errors.email?.message} readOnly={accountCreated} />
          <PasswordField registration={register("password")} error={errors.password?.message} visible={showPassword} autoComplete="new-password" readOnly={accountCreated} help={i18n("auth.password.help")} />
          <PasswordField id="password-confirmation" registration={register("passwordConfirmation")} error={errors.passwordConfirmation?.message} label={i18n("auth.password.confirmation")} visible={showPassword} onVisibilityChange={setShowPassword} autoComplete="new-password" readOnly={accountCreated} />
          {error && <p className={`${styles.error} text-description`} role="alert">{accountCreated ? i18n("auth.signup.retryAfterCreated", { error: i18n(error) }) : i18n(error)}</p>}
          <SubmitButton submitting={isSubmitting} pendingLabel={accountCreated ? i18n("auth.signup.loginSubmitting") : i18n("auth.signup.submitting")}>{actionLabel}</SubmitButton>
        </fieldset>
      </form>
    </section>
  )
}
