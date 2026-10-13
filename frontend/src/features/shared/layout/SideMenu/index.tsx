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
          <SideMenuItem tab="inbox" inboxCount={inboxCount} />
          <SideMenuItem tab="weekly-plan" />
          <SideMenuItem tab="today" inboxCount={inboxCount} />
        </div>
        <div className={styles.group} role="group" aria-labelledby="manage-label">
          <p className={`${styles.groupLabel} text-caption`} id="manage-label">{i18n("sidebar.group.manage")}</p>
          <SideMenuItem tab="projects" inboxCount={inboxCount} />
          <SideMenuItem tab="tasks" />
          <SideMenuItem tab="schedules" inboxCount={inboxCount} />
        </div>
        <div className={styles.group} role="group" aria-labelledby="team-label">
          <p className={`${styles.groupLabel} text-caption`} id="team-label">{i18n("sidebar.group.team")}</p>
          <SideMenuItem tab="members" />
        </div>
        <div className={styles.group} role="group" aria-labelledby="analysis-label">
          <p className={`${styles.groupLabel} text-caption`} id="analysis-label">{i18n("sidebar.group.insights")}</p>
          <SideMenuItem tab="metrics" inboxCount={inboxCount} />
        </div>
        <div className={styles.profile}>
          <SideMenuItem tab="billing" />
          <SideMenuItem tab="profile" inboxCount={inboxCount} />
        </div>
      </nav>
    </aside>
  )
}

export default SideMenu
