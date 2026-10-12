import type { ReactNode } from "react"
import { Link } from "react-router"
import DisplaySwitcher from "~/features/display/DisplaySwitcher"
import LanguageSwitcher from "~/features/i18n/LanguageSwitcher"
import Wordmark from "~/features/shared/components/Wordmark"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

export default function AuthPageFrame({ children }: { children: ReactNode }) {
  return (
    <main className={styles.page}>
      <header className={styles.header}>
        <Link className={`${styles.brand} ${controls.focusRing}`} to="/login">
          <Wordmark className={styles.wordmark} width={200} height={25} />
        </Link>
        <div className={styles.preferences}><LanguageSwitcher /><DisplaySwitcher /></div>
      </header>
      <div className={styles.container}>{children}</div>
    </main>
  )
}
