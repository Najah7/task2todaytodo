import { useId } from "react"
import wordmark from "~/assets/service_name.webp"
import styles from "./index.module.css"

type Props = {
  className?: string
  width?: number
  height?: number
}

export default function Wordmark({ className = "", width = 176, height = 22 }: Props) {
  const id = useId()
  const inkFilter = `${id}-ink-filter`
  const accentFilter = `${id}-accent-filter`
  const inkMask = `${id}-ink-mask`
  const accentMask = `${id}-accent-mask`

  return (
    <svg className={`${styles.wordmark} ${className}`} width={width} height={height} viewBox="0 0 1568 195" role="img" aria-label="Task2TodayTodo">
      <defs>
        {/* Separate ink and orange from the white-backed artwork without changing its geometry. */}
        <filter id={inkFilter} x="0" y="0" width="100%" height="100%" colorInterpolationFilters="sRGB">
          <feColorMatrix type="matrix" values="0 0 0 0 1  0 0 0 0 1  0 0 0 0 1  -1.095 0 0 0 1.095" />
        </filter>
        <filter id={accentFilter} x="0" y="0" width="100%" height="100%" colorInterpolationFilters="sRGB">
          <feColorMatrix type="matrix" values="0 0 0 0 1  0 0 0 0 1  0 0 0 0 1  1.138 0 -1.138 0 0" />
        </filter>
        <mask id={inkMask} maskUnits="userSpaceOnUse" x="0" y="0" width="1568" height="195" style={{ maskType: "alpha" }}>
          <image href={wordmark} width="1568" height="195" filter={`url(#${inkFilter})`} />
        </mask>
        <mask id={accentMask} maskUnits="userSpaceOnUse" x="0" y="0" width="1568" height="195" style={{ maskType: "alpha" }}>
          <image href={wordmark} width="1568" height="195" filter={`url(#${accentFilter})`} />
        </mask>
      </defs>
      <rect width="1568" height="195" className={styles.ink} mask={`url(#${inkMask})`} />
      <rect width="1568" height="195" className={styles.accent} mask={`url(#${accentMask})`} />
    </svg>
  )
}
