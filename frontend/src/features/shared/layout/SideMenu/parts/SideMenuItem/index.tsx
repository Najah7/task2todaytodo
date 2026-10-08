import { NavLink } from "react-router"
import type { MessageKey } from "~/features/i18n/messages"
import { useI18n } from "~/features/i18n/hooks"
import type { Tab } from "~/store/tab"
import MenuIcon from "../MenuIcon"
import styles from "./index.module.css"

type Props = {
  tab: Tab
  inboxCount?: number
}

const labelKeys: Record<Tab, MessageKey> = {
  today: "sidebar.item.today",
  inbox: "sidebar.item.inbox",
  projects: "sidebar.item.projects",
  tasks: "sidebar.item.tasks",
  schedules: "sidebar.item.schedules",
  members: "sidebar.item.members",
  kpi: "sidebar.item.kpi",
  profile: "sidebar.item.profile",
}

export const SideMenuItem = ({ tab, inboxCount = 0 }: Props) => {
  const i18n = useI18n()

  return (
    <NavLink
      to={`/${tab}`}
      className={({ isActive }) =>
        `${styles.navItem} ${isActive ? `${styles.active} text-body-strong` : "text-body"}`
      }
    >
      <MenuIcon tab={tab} />
      <span>{i18n(labelKeys[tab])}</span>
      {tab === "inbox" && inboxCount > 0 && (
        <span className={`${styles.count} text-tag`} aria-label={i18n("sidebar.inbox.waitingCount", { count: inboxCount })}>
          {inboxCount}
        </span>
      )}
    </NavLink>
  )
}
