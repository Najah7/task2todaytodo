import { useContext } from "react"
import PageHeading from "~/features/shared/components/PageHeading"
import { useI18n } from "~/features/i18n/hooks"
import { InboxActionProvider } from "~/features/inbox/providers/InboxActionProvider"
import { InboxActionContext } from "~/features/inbox/providers/InboxActionProvider/context"
import { InboxAPIDataProvider } from "~/features/inbox/providers/InboxAPIDataProvider"
import { InboxAPIDataContext } from "~/features/inbox/providers/InboxAPIDataProvider/context"
import { InboxNavigationProvider } from "~/features/inbox/providers/InboxNavigationProvider"
import controls from "~/styles/controls.module.css"
import InboxQuickCreate from "~/features/inbox/components/InboxQuickCreate"
import InboxTabs from "~/features/inbox/components/InboxTabs"
import InboxToolbar from "~/features/inbox/components/InboxToolbar"
import InboxList from "~/features/inbox/components/InboxList"
import InboxPagination from "~/features/inbox/components/InboxPagination"
import styles from "./index.module.css"

export default function InboxPage() {
  return (
    <InboxAPIDataProvider>
      <InboxActionProvider>
        <InboxNavigationProvider>
          <InboxPageContent />
        </InboxNavigationProvider>
      </InboxActionProvider>
    </InboxAPIDataProvider>
  )
}

export function InboxPageContent() {
  const i18n = useI18n()
  const data = useContext(InboxAPIDataContext)!
  const actions = useContext(InboxActionContext)!
  const hasData = Boolean(data.inbox)
  const userReady = Boolean(data.userQuery.data && isValidTimezone(data.timezone))
  const listReady = hasData && userReady && data.state !== "forbidden"
  const userLoading = !userReady && data.userQuery.isPending
  const userError = !userReady && !data.userQuery.isPending

  return (
    <section className={styles.page} aria-labelledby="inbox-heading">
      <header className={styles.header}>
        <PageHeading id="inbox-heading" messageKey="page.inbox.title" descriptionKey="inbox.description" />
      </header>
      {data.state !== "forbidden" && userReady && !data.projectsQuery.isPending && <InboxQuickCreate />}
      {data.projectsQuery.error && !data.projectsQuery.data && (
        <div className={styles.notice} role="alert">
          <span className="text-body">{i18n("inbox.optionsError")}</span>
          <button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} type="button" onClick={() => void actions.retryOptions()}>{i18n("inbox.retry")}</button>
        </div>
      )}
      {data.state === "forbidden" && <p className={styles.message} role="alert">{i18n("inbox.forbidden")}</p>}
      {userLoading && <p className={styles.message} role="status">{i18n("inbox.loading")}</p>}
      {userError && (
        <div className={styles.message} role="alert">
          <p className="text-body">{i18n(data.userQuery.data ? "inbox.timezoneError" : "inbox.loadError")}</p>
          <button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} type="button" onClick={() => void actions.retry()}>{i18n("inbox.retry")}</button>
        </div>
      )}
      {!userLoading && !userError && data.state === "loading" && !hasData && <p className={styles.message} role="status">{i18n("inbox.loading")}</p>}
      {!userLoading && !userError && data.state === "error" && !hasData && (
        <div className={styles.message} role="alert">
          <p className="text-body">{i18n("inbox.loadError")}</p>
          <button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} type="button" onClick={() => void actions.retry()}>{i18n("inbox.retry")}</button>
        </div>
      )}
      {listReady && <>
        {data.inboxQuery.error && <div className={styles.backgroundError} role="status"><span className="text-caption">{i18n("inbox.loadError")}</span><button className={styles.retryLink} type="button" onClick={() => void actions.retry()}>{i18n("inbox.retry")}</button></div>}
        <InboxTabs />
        <section className={`${controls.card} ${styles.card}`} aria-label={i18n("inbox.list.title")}>
          <InboxToolbar />
          <InboxList />
          <InboxPagination />
        </section>
      </>}
    </section>
  )
}

function isValidTimezone(timezone: string) {
  if (!timezone) return false
  try {
    new Intl.DateTimeFormat("en", { timeZone: timezone })
    return true
  } catch {
    return false
  }
}
