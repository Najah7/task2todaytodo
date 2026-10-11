import type { ReactNode } from "react"
import { useContext } from "react"
import { useI18n, type MessageKey } from "~/features/i18n/hooks"
import { ScheduleAPIDataContext } from "~/features/schedule/providers/ScheduleAPIDataProvider/context"
import PageHeading from "~/features/shared/components/PageHeading"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

export default function ScheduleFormBoundary({ heading, loadError, children }: { heading: MessageKey; loadError: MessageKey; children: ReactNode }) {
  const i18n = useI18n()
  const data = useContext(ScheduleAPIDataContext)!
  if (data.state === "ready") return children

  const message = data.state === "loading" ? "schedules.loading" : data.state === "forbidden" ? "schedules.forbidden" : loadError
  return (
    <section className={styles.state} aria-labelledby="schedule-form-boundary-heading">
      <PageHeading id="schedule-form-boundary-heading" messageKey={heading} />
      <p className="text-body" role={data.state === "loading" ? "status" : "alert"}>{i18n(message)}</p>
      {data.state === "error" && <button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} type="button" onClick={() => void data.refetch()}>{i18n("schedules.form.reload")}</button>}
    </section>
  )
}
