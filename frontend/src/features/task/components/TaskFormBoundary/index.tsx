import { useContext, type ReactNode } from "react"
import { useI18n, type MessageKey } from "~/features/i18n/hooks"
import { TaskAPIDataContext } from "~/features/task/providers/TaskAPIDataProvider/context"
import { TaskActionContext } from "~/features/task/providers/TaskActionProvider/context"
import PageHeading from "~/features/shared/components/PageHeading"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

type Props = {
  heading: MessageKey
  loadError: MessageKey
  children: ReactNode
}

export default function TaskFormBoundary({ heading, loadError, children }: Props) {
  const i18n = useI18n()
  const data = useContext(TaskAPIDataContext)!
  const actions = useContext(TaskActionContext)!
  if (data.state === "ready") return children

  const message: MessageKey = data.state === "loading"
    ? "tasks.loading"
    : data.state === "forbidden"
      ? "tasks.form.forbidden"
      : loadError
  const retryLabel: MessageKey = "tasks.retry"

  return (
    <section className={styles.pageState} aria-labelledby="task-form-boundary-heading">
      <PageHeading id="task-form-boundary-heading" messageKey={heading} />
      <p className="text-body" role={data.state === "loading" ? "status" : "alert"}>{i18n(message)}</p>
      {data.state === "error" && <button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} type="button" onClick={() => void actions.retry()}>{i18n(retryLabel)}</button>}
    </section>
  )
}
