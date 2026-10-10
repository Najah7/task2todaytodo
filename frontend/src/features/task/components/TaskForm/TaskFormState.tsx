import { useI18n, type MessageKey } from "~/features/i18n/hooks"
import PageHeading from "~/features/shared/components/PageHeading"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

type Props = { heading: MessageKey; message: MessageKey; status?: boolean; retry?: () => void; retryLabel?: MessageKey }

export default function TaskFormState({ heading, message, status = false, retry, retryLabel = "tasks.retry" }: Props) {
  const i18n = useI18n()
  return (
    <section className={styles.pageState}>
      <PageHeading messageKey={heading} />
      <p className="text-body" role={status ? "status" : "alert"}>{i18n(message)}</p>
      {retry && <button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} type="button" onClick={retry}>{i18n(retryLabel)}</button>}
    </section>
  )
}
