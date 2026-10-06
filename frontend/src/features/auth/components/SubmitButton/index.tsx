import type { ReactNode } from "react"
import controls from "~/styles/controls.module.css"

type SubmitButtonProps = {
  submitting: boolean
  pendingLabel: string
  children: ReactNode
}

export default function SubmitButton({ submitting, pendingLabel, children }: SubmitButtonProps) {
  return (
    <button className={`${controls.button} ${controls.primaryButton} ${controls.focusRing} text-button`} type="submit" disabled={submitting}>
      {submitting ? pendingLabel : children}
    </button>
  )
}
