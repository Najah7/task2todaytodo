import type { UseFormRegisterReturn } from "react-hook-form"
import controls from "~/styles/controls.module.css"
import { useI18n } from "~/features/i18n/hooks"
import { getValidationMessageKey } from "~/features/auth/lib/validationMessage"
import styles from "./index.module.css"

type EmailFieldProps = {
  registration: UseFormRegisterReturn
  error?: string
  readOnly?: boolean
}

export default function EmailField({ registration, error, readOnly = false }: EmailFieldProps) {
  const i18n = useI18n()

  return (
    <div className={styles.field}>
      <label htmlFor="email" className="text-field-label">{i18n("auth.email.label")}</label>
      <input {...registration} className={controls.input} id="email" type="email" autoComplete="email" placeholder="you@example.com" required readOnly={readOnly} aria-invalid={Boolean(error)} aria-describedby={error ? "email-error" : undefined} />
      {error && <p id="email-error" className={`${styles.error} text-description`} role="alert">{i18n(getValidationMessageKey(error))}</p>}
    </div>
  )
}
