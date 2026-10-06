import type { ReactNode } from "react"
import { useI18n } from "~/features/i18n/hooks"
import Header from "~/features/shared/layout/Header"
import SideMenu from "~/features/shared/layout/SideMenu"
import styles from "./App.module.css"

function App({ children }: { children: ReactNode }) {
  const i18n = useI18n()

  return (
    <div className={styles.layout}>
      <a className={styles.skipLink} href="#main-content">{i18n("common.skipToContent")}</a>
      <Header />
      <SideMenu />
      <main className={styles.content} id="main-content" tabIndex={-1}>
        {children}
      </main>
    </div>
  )
}

export default App
