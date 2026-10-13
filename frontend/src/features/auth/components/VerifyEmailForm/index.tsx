import { useState } from "react"
import { useQueryClient } from "@tanstack/react-query"
import { Link, useNavigate } from "react-router"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import AuthActionPanel from "~/features/auth/components/AuthActionPanel"
import PasswordField from "~/features/auth/components/PasswordField"
import { postEmailVerificationConfirm } from "~/api/generated/auth"
import { getAuthLinkErrorMessage } from "~/features/auth/errors"
import { useEmailActionToken } from "~/features/auth/hooks/useEmailActionToken"
import { savePersonalAccessToken } from "~/features/auth/lib/localStorage"
import { useI18n, type MessageKey } from "~/features/i18n/hooks"
import { verifyEmailSchema, type VerifyEmailFormValues } from "./schema"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"
import { DEFAULT_TAB } from "~/store/tab"

export default function VerifyEmailForm() {
  const i18n = useI18n()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const token = useEmailActionToken()
  const [visible, setVisible] = useState(false)
  const [error, setError] = useState<MessageKey | null>(null)
  const { register, handleSubmit, formState: { errors, isSubmitting } } = useForm<VerifyEmailFormValues>({
    resolver: zodResolver(verifyEmailSchema),
    defaultValues: { password: "" },
  })

  async function onSubmit({ password }: VerifyEmailFormValues) {
    if (!token || isSubmitting) return
    setError(null)
    try {
      const result = await postEmailVerificationConfirm({ token, password })
      if ("personal_access_token" in result && typeof result.personal_access_token === "string" && result.personal_access_token.trim()) {
        savePersonalAccessToken(result.personal_access_token)
        await queryClient.cancelQueries()
        queryClient.clear()
        navigate(`/${DEFAULT_TAB}`, { replace: true })
      } else {
        navigate("/login?verified=1", { replace: true })
      }
    } catch (cause) {
      setError(getAuthLinkErrorMessage(cause, "auth.emailVerification.invalidLink"))
    }
  }

  return (
    <AuthActionPanel title={i18n("auth.emailVerification.title")}>
      <form className={styles.fields} onSubmit={handleSubmit(onSubmit)} noValidate aria-busy={isSubmitting}>
        <p className={`${styles.note} text-description`}>{i18n(token ? "auth.emailVerification.description" : "auth.emailVerification.missingToken")}</p>
        <PasswordField registration={register("password")} error={errors.password?.message} visible={visible} onVisibilityChange={setVisible} autoComplete="current-password" />
        {error && <p className={`${styles.error} text-description`} role="alert">{i18n(error)}</p>}
        <button className={`${controls.button} ${controls.primaryButton} ${controls.focusRing} text-button`} type="submit" disabled={!token || isSubmitting}>
          {isSubmitting ? i18n("auth.emailVerification.confirming") : i18n("auth.emailVerification.confirm")}
        </button>
        <Link className={`${styles.link} text-caption`} to="/login">{i18n("auth.emailVerification.loginLink")}</Link>
        <Link className={`${styles.link} text-caption`} to="/signup">{i18n("auth.emailVerification.resend")}</Link>
      </form>
    </AuthActionPanel>
  )
}
