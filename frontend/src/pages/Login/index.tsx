import { Link } from "react-router"
import LoginForm from "~/features/auth/components/LoginForm"
import DisplaySwitcher from "~/features/display/DisplaySwitcher"
import LanguageSwitcher from "~/features/i18n/LanguageSwitcher"
import Wordmark from "~/features/shared/components/Wordmark"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"
import { useI18n } from "~/features/i18n/hooks"

export default function LoginPage() {
  const i18n = useI18n()

  return (
    <main className={styles.page}>
      <header className={styles.header}>
        <Link className={`${styles.brand} ${controls.focusRing}`} to="/today">
          <Wordmark className={styles.wordmark} width={200} height={25} />
        </Link>
        <div className={styles.preferences}>
          <LanguageSwitcher />
          <DisplaySwitcher />
        </div>
      </header>
      <div className={styles.container}>
        <LoginForm />
        <aside className={`${controls.card} ${styles.switchCard}`}>
          <p className={`${styles.note} text-description`}>{i18n("auth.login.newHere")}</p>
          <Link className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} to="/signup">
            {i18n("auth.login.signup")}
          </Link>
        </aside>
        <Link className={`${styles.backLink} ${controls.focusRing} text-caption`} to="/today">{i18n("common.backToToday")}</Link>
      </div>
    </main>
  )
}
