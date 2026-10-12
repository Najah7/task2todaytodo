import { useQuery } from "@tanstack/react-query"
import { Link, useSearchParams } from "react-router"
import { getRolesCapabilities } from "~/api/generated/projects"
import { useI18n } from "~/features/i18n/hooks"
import MemberRoleTable from "~/features/project/components/MemberRoleTable"
import PageHeading from "~/features/shared/components/PageHeading"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

export default function MembersRoleHelpPage() {
  const i18n = useI18n()
  const [searchParams] = useSearchParams()
  const query = useQuery({ queryKey: ["roleCapabilities"], queryFn: getRolesCapabilities, retry: false })
  const projectId = searchParams.get("projectId")
  const returnTo = projectId ? `/members?projectId=${encodeURIComponent(projectId)}` : "/members"

  return (
    <section className={styles.page} aria-labelledby="role-help-heading">
      <header className={styles.header}>
        <PageHeading id="role-help-heading" messageKey="members.help.title" descriptionKey="members.help.description" />
        <Link className={`${controls.button} ${controls.neutralButton} ${controls.focusRing} text-button-small`} to={returnTo}>{i18n("members.help.back")}</Link>
      </header>
      {query.isPending ? <p className={`${styles.state} text-body`}>{i18n("members.help.loading")}</p> : query.isError ? (
        <div className={`${controls.card} ${styles.state}`} role="alert">
          <p className="text-body">{i18n("members.help.loadError")}</p>
          <button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} type="button" onClick={() => void query.refetch()}>{i18n("members.retry")}</button>
        </div>
      ) : query.data ? (
        <section className={`${controls.card} ${styles.matrix}`} aria-label={i18n("members.help.title")}>
          <MemberRoleTable
            roles={(query.data.roles ?? []).flatMap((role) => role.id ? [{ id: role.id }] : [])}
            capabilities={(query.data.capabilities ?? []).flatMap((capability) => capability.id ? [{
              id: capability.id,
              owner: capability.owner ?? false,
              admin: capability.admin ?? false,
              editor: capability.editor ?? false,
              viewer: capability.viewer ?? false,
            }] : [])}
          />
        </section>
      ) : null}
    </section>
  )
}
