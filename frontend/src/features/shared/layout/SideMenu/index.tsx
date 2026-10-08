import { useI18n } from "~/features/i18n/hooks"
import styles from "./index.module.css"
import { SideMenuItem } from "./parts/SideMenuItem"

type Props = {
  inboxCount?: number
}

const SideMenu = ({ inboxCount = 0 }: Props) => {
  const i18n = useI18n()

  return (
    <aside className={styles.sidebar}>
      <nav className={styles.navigation} aria-label={i18n("common.navigation")}>
        <div className={styles.group}>
          <SideMenuItem tab="today" inboxCount={inboxCount} />
          <SideMenuItem tab="inbox" inboxCount={inboxCount} />
        </div>
        <div className={styles.group} role="group" aria-labelledby="manage-label">
          <p className={`${styles.groupLabel} text-caption`} id="manage-label">{i18n("sidebar.group.manage")}</p>
          <SideMenuItem tab="projects" inboxCount={inboxCount} />
          <SideMenuItem tab="tasks" />
          <SideMenuItem tab="calendar" inboxCount={inboxCount} />
        </div>
        <div className={styles.group} role="group" aria-labelledby="analysis-label">
          <p className={`${styles.groupLabel} text-caption`} id="analysis-label">{i18n("sidebar.group.insights")}</p>
          <SideMenuItem tab="kpi" inboxCount={inboxCount} />
        </div>
        <div className={styles.profile}>
          <SideMenuItem tab="profile" inboxCount={inboxCount} />
        </div>
      </nav>
    </aside>
  )
}

export default SideMenu
