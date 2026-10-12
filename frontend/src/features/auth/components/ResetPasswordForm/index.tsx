import { useState } from "react"
import { Link, useNavigate } from "react-router"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useQueryClient } from "@tanstack/react-query"
import AuthActionPanel from "~/features/auth/components/AuthActionPanel"
import PasswordField from "~/features/auth/components/PasswordField"
import { postPasswordResetConfirm } from "~/api/generated/auth"
import { getAuthLinkErrorMessage } from "~/features/auth/errors"
import { useEmailActionToken } from "~/features/auth/hooks/useEmailActionToken"
import { clearPersonalAccessToken } from "~/features/auth/lib/localStorage"
import { useI18n, type MessageKey } from "~/features/i18n/hooks"
import { resetPasswordSchema, type ResetPasswordFormValues } from "./schema"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

export default function ResetPasswordForm() {
  const i18n = useI18n()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const token = useEmailActionToken()
  const [visible, setVisible] = useState(false)
  const [error, setError] = useState<MessageKey | null>(null)
  const { register, handleSubmit, formState: { errors, isSubmitting } } = useForm<ResetPasswordFormValues>({
    resolver: zodResolver(resetPasswordSchema),
    defaultValues: { password: "", passwordConfirmation: "" },
  })

  async function onSubmit(values: ResetPasswordFormValues) {
    if (!token || isSubmitting) return
    setError(null)
    try {
      await postPasswordResetConfirm({ token, password: values.password, password_confirmation: values.passwordConfirmation })
      await queryClient.cancelQueries()
      queryClient.clear()
      clearPersonalAccessToken()
      navigate("/login?password-reset=1", { replace: true })
    } catch (cause) {
      setError(getAuthLinkErrorMessage(cause, "auth.passwordReset.invalidLink"))
    }
  }

  return (
    <AuthActionPanel title={i18n("auth.passwordReset.title")}>
      <form className={styles.fields} onSubmit={handleSubmit(onSubmit)} noValidate aria-busy={isSubmitting}>
        <p className={`${styles.note} text-description`}>{i18n(token ? "auth.passwordReset.description" : "auth.passwordReset.missingToken")}</p>
        <PasswordField registration={register("password")} error={errors.password?.message} visible={visible} autoComplete="new-password" help={i18n("auth.password.help")} />
        <PasswordField id="password-confirmation" registration={register("passwordConfirmation")} error={errors.passwordConfirmation?.message} label={i18n("auth.password.confirmation")} visible={visible} onVisibilityChange={setVisible} autoComplete="new-password" />
        {error && <p className={`${styles.error} text-description`} role="alert">{i18n(error)}</p>}
        <button className={`${controls.button} ${controls.primaryButton} ${controls.focusRing} text-button`} type="submit" disabled={!token || isSubmitting}>
          {isSubmitting ? i18n("auth.passwordReset.saving") : i18n("auth.passwordReset.submit")}
        </button>
        <Link className={`${styles.link} text-caption`} to="/request-password-reset">{i18n("auth.passwordReset.requestAgain")}</Link>
        <Link className={`${styles.link} text-caption`} to="/login">{i18n("auth.emailVerification.loginLink")}</Link>
      </form>
    </AuthActionPanel>
  )
}
