import { Link } from "react-router"
import DisplaySwitcher from "~/features/display/DisplaySwitcher"
import LanguageSwitcher from "~/features/i18n/LanguageSwitcher"
import Wordmark from "~/features/shared/components/Wordmark"
import { DEFAULT_TAB } from "~/store/tab"
import styles from "./index.module.css"

export default function Header() {
  return (
    <header className={styles.header}>
      <Link className={styles.brand} to={`/${DEFAULT_TAB}`}>
        <Wordmark />
      </Link>
      <div className={styles.preferences}>
        <LanguageSwitcher />
        <DisplaySwitcher />
      </div>
    </header>
  )
}
