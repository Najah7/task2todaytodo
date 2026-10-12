import { useState } from "react"
import { useForm, type UseFormRegisterReturn } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { usePostSignup } from "~/api/generated/auth"
import { signupSchema, type SignupFormValues } from "./schema"
import { getAuthErrorMessage, getAuthFieldError } from "~/features/auth/errors"
import { postEmailVerificationResend } from "~/api/generated/auth"
import EmailField from "~/features/auth/components/EmailField"
import PasswordField from "~/features/auth/components/PasswordField"
import SubmitButton from "~/features/auth/components/SubmitButton"
import { useI18n, type MessageKey } from "~/features/i18n/hooks"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

function withClearError(registration: UseFormRegisterReturn, clear: () => void): UseFormRegisterReturn {
  return {
    ...registration,
    onChange: async (event) => {
      clear()
      await registration.onChange(event)
    },
  }
}

export default function SignupForm() {
  const i18n = useI18n()
  const signup = usePostSignup({ mutation: { retry: false, gcTime: 0 } })
  const { register, handleSubmit, setError: setFieldError, clearErrors, formState: { errors, isSubmitting } } = useForm<SignupFormValues>({
    resolver: zodResolver(signupSchema),
    defaultValues: { email: "", password: "", passwordConfirmation: "" },
  })
  const [showPassword, setShowPassword] = useState(false)
  const [deliveryRequested, setDeliveryRequested] = useState(false)
  const [email, setEmail] = useState("")
  const [resendPending, setResendPending] = useState(false)
  const [error, setError] = useState<MessageKey | null>(null)

  async function onSubmit(values: SignupFormValues) {
    if (isSubmitting) return
    setError(null)
    try {
      await signup.mutateAsync({ data: { email: values.email, password: values.password } })
      setEmail(values.email)
      setDeliveryRequested(true)
    } catch (cause) {
      const fieldError = getAuthFieldError(cause)
      if (fieldError) {
        setFieldError(fieldError.field, { type: "server", message: fieldError.translationKey })
        return
      }
      const errorKey = getAuthErrorMessage(cause)
      setError(errorKey)
    }
  }

  async function resend() {
    if (resendPending || !email) return
    setResendPending(true)
    try {
      await postEmailVerificationResend({ email })
      setError(null)
    } catch {
      setError("auth.error.generic")
    } finally {
      setResendPending(false)
    }
  }

  return (
    <section className={`${controls.card} ${styles.card}`} aria-labelledby="signup-title">
      <h1 id="signup-title" className={`${styles.title} text-login-title`}>{i18n("auth.signup.title")}</h1>
      {deliveryRequested ? (
        <div className={styles.fields}>
          <p className="text-description" role="status">{i18n("auth.emailVerification.guidance")}</p>
          <button className={`${controls.button} ${controls.primaryButton} ${controls.focusRing} text-button`} type="button" onClick={() => void resend()} disabled={resendPending}>
            {resendPending ? i18n("auth.emailVerification.resending") : i18n("auth.emailVerification.resend")}
          </button>
          {error && <p className={`${styles.error} text-description`} role="alert">{i18n(error)}</p>}
        </div>
      ) : (
      <form onSubmit={handleSubmit(onSubmit, () => setError(null))} noValidate aria-busy={isSubmitting}>
        <fieldset className={styles.fields} disabled={isSubmitting}>
          <EmailField registration={withClearError(register("email"), () => clearErrors("email"))} error={errors.email?.message} />
          <PasswordField registration={withClearError(register("password"), () => clearErrors("password"))} error={errors.password?.message} visible={showPassword} autoComplete="new-password" help={i18n("auth.password.help")} />
          <PasswordField id="password-confirmation" registration={register("passwordConfirmation")} error={errors.passwordConfirmation?.message} label={i18n("auth.password.confirmation")} visible={showPassword} onVisibilityChange={setShowPassword} autoComplete="new-password" />
          {error && <p className={`${styles.error} text-description`} role="alert">{i18n(error)}</p>}
          <SubmitButton submitting={isSubmitting} pendingLabel={i18n("auth.signup.submitting")}>{i18n("auth.signup.submit")}</SubmitButton>
        </fieldset>
      </form>
      )}
    </section>
  )
}
