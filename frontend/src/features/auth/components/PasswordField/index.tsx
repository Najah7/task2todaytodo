import type { UseFormRegisterReturn } from "react-hook-form"
import controls from "~/styles/controls.module.css"
import { useI18n } from "~/features/i18n/hooks"
import { getValidationMessageKey } from "~/features/auth/lib/validationMessage"
import styles from "./index.module.css"

type PasswordFieldProps = {
  id?: string
  registration: UseFormRegisterReturn
  error?: string
  label?: string
  visible: boolean
  onVisibilityChange?: (visible: boolean) => void
  autoComplete: "new-password" | "current-password"
  readOnly?: boolean
  help?: string
}

export default function PasswordField({ id = "password", registration, error, label, visible, onVisibilityChange, autoComplete, readOnly, help }: PasswordFieldProps) {
  const i18n = useI18n()
  const description = [help && `${id}-help`, error && `${id}-error`].filter(Boolean).join(" ") || undefined

  return (
    <div className={styles.field}>
      <label htmlFor={id} className="text-field-label">{label ?? i18n("auth.password.label")}</label>
      <input {...registration} className={controls.input} id={id} type={visible ? "text" : "password"} autoComplete={autoComplete} required readOnly={readOnly} aria-invalid={Boolean(error)} aria-describedby={description} />
      {help && <p id={`${id}-help`} className={`${styles.note} text-caption`}>{help}</p>}
      {error && <p id={`${id}-error`} className={`${styles.error} text-description`} role="alert">{i18n(getValidationMessageKey(error))}</p>}
      {onVisibilityChange && (
        <label className={`${styles.passwordToggle} text-description`}>
          <input className={`${styles.checkbox} ${controls.focusRing}`} type="checkbox" checked={visible} onChange={(event) => onVisibilityChange(event.target.checked)} />
          {i18n("auth.password.show")}
        </label>
      )}
    </div>
  )
}
