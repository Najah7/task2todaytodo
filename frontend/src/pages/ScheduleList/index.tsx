import { useContext } from "react"
import { useI18n } from "~/features/i18n/hooks"
import ScheduleCalendar from "~/features/schedule/components/ScheduleCalendar"
import { ScheduleActionProvider } from "~/features/schedule/providers/ScheduleActionProvider"
import { ScheduleAPIDataProvider } from "~/features/schedule/providers/ScheduleAPIDataProvider"
import { ScheduleAPIDataContext } from "~/features/schedule/providers/ScheduleAPIDataProvider/context"
import { ScheduleNavigationProvider } from "~/features/schedule/providers/ScheduleNavigationProvider"
import { ScheduleNavigationContext } from "~/features/schedule/providers/ScheduleNavigationProvider/context"
import PageHeading from "~/features/shared/components/PageHeading"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

export default function ScheduleList() {
  return (
    <ScheduleAPIDataProvider list>
      <ScheduleNavigationProvider>
        <ScheduleActionProvider>
          <ScheduleListContent />
        </ScheduleActionProvider>
      </ScheduleNavigationProvider>
    </ScheduleAPIDataProvider>
  )
}

function ScheduleListContent() {
  const i18n = useI18n()
  const data = useContext(ScheduleAPIDataContext)!
  const navigation = useContext(ScheduleNavigationContext)!
  return (
    <section className={styles.page} aria-labelledby="schedules-heading">
      <header className={styles.header}>
        <PageHeading id="schedules-heading" messageKey="page.schedules.title" descriptionKey="schedules.description" />
        <button className={`${controls.button} ${controls.primaryButton} ${controls.focusRing} text-button`} type="button" onClick={() => navigation.create()}>{i18n("schedules.create")}</button>
      </header>
      {data.state === "loading" && <p className="text-body" role="status">{i18n("schedules.loading")}</p>}
      {data.state === "forbidden" && <p className="text-body" role="alert">{i18n("schedules.forbidden")}</p>}
      {data.state === "error" && <div className={styles.errorState} role="alert"><p className="text-body">{i18n("schedules.loadError")}</p><button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} type="button" onClick={() => void data.refetch()}>{i18n("schedules.retry")}</button></div>}
      {data.state === "ready" && <ScheduleCalendar />}
    </section>
  )
}
