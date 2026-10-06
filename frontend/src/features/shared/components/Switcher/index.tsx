import { useId, type ReactNode } from "react"
import styles from "./index.module.css"

type Props = {
  label: string
  leftLabel: string
  rightLabel: string
  left: ReactNode
  right: ReactNode
  rightSelected: boolean
  onChange: (rightSelected: boolean) => void
  disabled?: boolean
}

export default function Switcher({ label, leftLabel, rightLabel, left, right, rightSelected, onChange, disabled = false }: Props) {
  const descriptionId = useId()

  return (
    <button
      type="button"
      role="switch"
      className={`${styles.switcher} text-switcher`}
      data-selected={rightSelected ? "right" : "left"}
      aria-label={label}
      aria-checked={rightSelected}
      aria-describedby={descriptionId}
      disabled={disabled}
      onClick={() => onChange(!rightSelected)}
      onKeyDown={(event) => {
        if (event.key === "ArrowLeft" || event.key === "Home") {
          event.preventDefault()
          onChange(false)
        } else if (event.key === "ArrowRight" || event.key === "End") {
          event.preventDefault()
          onChange(true)
        }
      }}
    >
      <span className={styles.track} aria-hidden="true">
        <span className={styles.option}>{left}</span>
        <span className={styles.option}>{right}</span>
        <span className={styles.thumb}>
          <span className={styles.thumbLeft}>{left}</span>
          <span className={styles.thumbRight}>{right}</span>
        </span>
      </span>
      <span id={descriptionId} className={styles.description}>{rightSelected ? rightLabel : leftLabel}</span>
    </button>
  )
}
