import { useI18n, type MessageKey } from "~/features/i18n/hooks"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

export type MemberRosterItem = {
  key: string
  name: string
  email: string
  roleId: string
  status: "owner" | "accepted" | "pending" | "expired"
  expirationLabel?: string
  canEditRole: boolean
  canRemove: boolean
}

type Props = {
  items: MemberRosterItem[]
  roles: { id: string }[]
  canManageInvitations: boolean
  onRoleChange: (member: MemberRosterItem, roleId: string) => void
  onRemove: (member: MemberRosterItem) => void
  onResend: (member: MemberRosterItem) => void
  onRevoke: (member: MemberRosterItem) => void
}

export default function MemberRoster({ items, roles, canManageInvitations, onRoleChange, onRemove, onResend, onRevoke }: Props) {
  const i18n = useI18n()

  return (
    <div className={styles.scroll}>
      <table className={styles.table}>
        <thead>
          <tr>
            <th scope="col" className="text-label">{i18n("members.column.person")}</th>
            <th scope="col" className="text-label">{i18n("members.column.role")}</th>
            <th scope="col" className="text-label">{i18n("members.column.status")}</th>
            <th scope="col" className="text-label">{i18n("members.column.actions")}</th>
          </tr>
        </thead>
        <tbody>
          {items.map((member) => (
            <tr key={member.key}>
              <th scope="row" className={styles.person}>
                <span className={styles.avatar} aria-hidden="true">{member.name.trim().charAt(0).toLocaleUpperCase() || "?"}</span>
                <span className={styles.personText}>
                  <span className="text-body">{member.name || member.email}</span>
                  {member.name && <span className="text-caption">{member.email}</span>}
                </span>
              </th>
              <td>
                <span className={styles.mobileLabel}>{i18n("members.column.role")}</span>
                {member.status === "owner" || (member.status !== "accepted" || !member.canEditRole) ? (
                  <span className="text-body">{i18n(`members.role.${member.roleId}` as MessageKey)}</span>
                ) : (
                  <label>
                    <span className={styles.srOnly}>{i18n("members.changeRole", { name: member.name || member.email })}</span>
                    <select className={`${controls.select} ${controls.focusRing}`} value={member.roleId} onChange={(event) => onRoleChange(member, event.currentTarget.value)}>
                      {roles.map((role) => <option key={role.id} value={role.id}>{i18n(`members.role.${role.id}` as MessageKey)}</option>)}
                    </select>
                  </label>
                )}
              </td>
              <td>
                <span className={styles.mobileLabel}>{i18n("members.column.status")}</span>
                <span className={`${styles.status} ${styles[member.status]}`}>{i18n(`members.status.${member.status}` as MessageKey)}</span>
                {member.expirationLabel && <span className={styles.expiration}>{member.expirationLabel}</span>}
              </td>
              <td>
                <span className={styles.mobileLabel}>{i18n("members.column.actions")}</span>
                <div className={styles.actions}>
                  {(member.status === "pending" || member.status === "expired") && canManageInvitations && member.canRemove && (
                    <>
                      <button className={`${styles.linkButton} ${controls.focusRing} text-button-small`} type="button" onClick={() => onResend(member)}>{i18n("members.inviteAgain")}</button>
                      <button className={`${styles.linkButton} ${controls.focusRing} text-button-small`} type="button" onClick={() => onRevoke(member)}>{i18n("members.revoke")}</button>
                    </>
                  )}
                  {member.status === "accepted" && member.canRemove && (
                    <button className={`${styles.linkButton} ${controls.focusRing} text-button-small`} type="button" onClick={() => onRemove(member)}>{i18n("members.remove")}</button>
                  )}
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
