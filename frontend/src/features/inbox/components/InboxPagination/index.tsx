import { useContext } from "react"
import { useI18n } from "~/features/i18n/hooks"
import { InboxAPIDataContext } from "~/features/inbox/providers/InboxAPIDataProvider/context"
import { InboxNavigationContext } from "~/features/inbox/providers/InboxNavigationProvider/context"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

export default function InboxPagination() {
  const i18n = useI18n()
  const { inbox, inboxQuery } = useContext(InboxAPIDataContext)!
  const { state, first, previous, next } = useContext(InboxNavigationContext)!
  if (!inbox) return null
  const count = inbox.counts[state.type]
  const emptyPage = inbox.items.length === 0 && Boolean(state.pageToken)
  const previousDisabled = !inbox.previous_page_token || inboxQuery.isLoading
  const nextDisabled = !inbox.next_page_token || inboxQuery.isLoading

  return <footer className={styles.footer}>
    {emptyPage && <div className={styles.recovery} role="status">
      <span className="text-body">{i18n("inbox.emptyPage")}</span>
      <button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} type="button" onClick={first}>{i18n("inbox.pagination.first")}</button>
    </div>}
    {!emptyPage && <><span className="text-caption">{i18n("inbox.sort.createdDesc")}</span><span className="text-caption">{count}</span></>}
    <nav className={styles.navigation} aria-label={i18n("inbox.pagination.label")}>
      <button className={`${controls.button} ${controls.neutralButton} ${controls.focusRing} text-button-small`} type="button" disabled={previousDisabled} onClick={() => previous(inbox.previous_page_token)}>{i18n("inbox.pagination.previous")}</button>
      <button className={`${controls.button} ${controls.neutralButton} ${controls.focusRing} text-button-small`} type="button" disabled={nextDisabled} onClick={() => next(inbox.next_page_token)}>{i18n("inbox.pagination.next")}</button>
    </nav>
  </footer>
}
