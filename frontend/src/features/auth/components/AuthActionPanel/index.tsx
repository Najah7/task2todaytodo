import type { ReactNode } from "react"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

export default function AuthActionPanel({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className={`${controls.card} ${styles.card}`} aria-labelledby="auth-action-title">
      <h1 id="auth-action-title" className={`${styles.title} text-login-title`}>{title}</h1>
      {children}
    </section>
  )
}
