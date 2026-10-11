import { useContext } from "react"
import { ListInboxType } from "~/api/generated/inbox"
import { useI18n } from "~/features/i18n/hooks"
import { InboxAPIDataContext } from "~/features/inbox/providers/InboxAPIDataProvider/context"
import { InboxNavigationContext } from "~/features/inbox/providers/InboxNavigationProvider/context"
import styles from "./index.module.css"

export default function InboxTabs() {
  const i18n = useI18n()
  const { inbox } = useContext(InboxAPIDataContext)!
  const { state, selectType } = useContext(InboxNavigationContext)!
  const tabs = [
    { type: ListInboxType.all, count: inbox?.counts.all ?? 0 },
    { type: ListInboxType.task, count: inbox?.counts.task ?? 0 },
    { type: ListInboxType.schedule, count: inbox?.counts.schedule ?? 0 },
  ] as const

  return <div className={styles.tabs} role="tablist" aria-label={i18n("page.inbox.title")}>
    {tabs.map(({ type, count }) => <button
      key={type}
      className={`${styles.tab} ${state.type === type ? styles.active : ""} text-body`}
      type="button"
      role="tab"
      aria-selected={state.type === type}
      onClick={() => selectType(type)}
    >
      <span>{i18n(`inbox.tab.${type}`)}</span>
      <span className={`${styles.count} text-numeric`} aria-label={i18n("inbox.tab.count", { count })}>{count}</span>
    </button>)}
  </div>
}
