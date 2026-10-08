import { useEffect, useId, useRef } from "react"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

type Props = {
  open: boolean
  title: string
  description: string
  confirmLabel: string
  cancelLabel: string
  destructive?: boolean
  busy?: boolean
  onConfirm: () => void
  onCancel: () => void
}

export default function ConfirmationDialog({
  open,
  title,
  description,
  confirmLabel,
  cancelLabel,
  destructive = false,
  busy = false,
  onConfirm,
  onCancel,
}: Props) {
  const dialogRef = useRef<HTMLDialogElement>(null)
  const id = useId()

  useEffect(() => {
    const dialog = dialogRef.current
    if (!dialog) return
    if (typeof dialog.showModal === "function") dialog.showModal()
    else dialog.setAttribute("open", "")
    return () => {
      if (typeof dialog.close === "function") dialog.close()
      else dialog.removeAttribute("open")
    }
  }, [open])

  if (!open) return null

  return (
    <dialog
      ref={dialogRef}
      className={styles.dialog}
      role="alertdialog"
      aria-labelledby={`${id}-title`}
      aria-describedby={`${id}-description`}
      aria-busy={busy}
      onCancel={(event) => {
        event.preventDefault()
        if (!busy) onCancel()
      }}
    >
      <div className={styles.content}>
        <h2 className="text-card-title" id={`${id}-title`}>{title}</h2>
        <p className={`${styles.description} text-body`} id={`${id}-description`}>{description}</p>
      </div>
      <div className={styles.actions}>
        <button
          type="button"
          className={`${controls.button} ${controls.neutralButton} ${controls.focusRing} text-body`}
          autoFocus
          disabled={busy}
          onClick={onCancel}
        >
          {cancelLabel}
        </button>
        <button
          type="button"
          className={`${controls.button} ${controls.primaryButton} ${controls.focusRing} ${destructive ? styles.destructive : ""} text-button`}
          disabled={busy}
          onClick={onConfirm}
        >
          {confirmLabel}
        </button>
      </div>
    </dialog>
  )
}
