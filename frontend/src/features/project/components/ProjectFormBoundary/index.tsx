import type { ReactNode } from "react"
import { useContext } from "react"
import { useI18n, type MessageKey } from "~/features/i18n/hooks"
import { ProjectActionContext } from "~/features/project/providers/ProjectActionProvider/context"
import { ProjectAPIDataContext } from "~/features/project/providers/ProjectAPIDataProvider/context"
import PageHeading from "~/features/shared/components/PageHeading"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

type Props = { heading: MessageKey; children: ReactNode }

export default function ProjectFormBoundary({ heading, children }: Props) {
  const i18n = useI18n()
  const { state, projectId } = useContext(ProjectAPIDataContext)!
  const { retry } = useContext(ProjectActionContext)!
  if (state === "ready") return children

  const message: MessageKey = state === "loading"
    ? "projects.loading"
    : state === "forbidden"
      ? "projects.error.forbidden"
      : "projects.form.loadError"
  const retryLabel: MessageKey = projectId !== undefined ? "projects.form.reload" : "projects.retry"

  return (
    <section className={styles.pageState} aria-labelledby="project-form-boundary-heading">
      <PageHeading id="project-form-boundary-heading" messageKey={heading} />
      <p className="text-body" role={state === "loading" ? "status" : "alert"}>{i18n(message)}</p>
      {state === "error" && <button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} type="button" onClick={() => void retry()}>{i18n(retryLabel)}</button>}
    </section>
  )
}
