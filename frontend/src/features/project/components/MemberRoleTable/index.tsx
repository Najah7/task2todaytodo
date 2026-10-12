import { useI18n, type MessageKey } from "~/features/i18n/hooks"
import styles from "./index.module.css"

export type MemberRoleCapability = {
  id: string
  owner: boolean
  admin: boolean
  editor: boolean
  viewer: boolean
}

export type MemberRole = { id: string }

type Props = {
  roles: MemberRole[]
  capabilities: MemberRoleCapability[]
}

const ACTION_LABELS: Record<string, MessageKey> = {
  "project.read": "members.help.resource.project",
  "project.update": "members.help.resource.projectUpdate",
  "project.delete": "members.help.resource.projectDelete",
  "project_member.read": "members.help.resource.membersRead",
  "project_member.invite": "members.help.resource.membersInvite",
  "project_member.manage_basic": "members.help.resource.membersBasic",
  "project_member.manage_admin": "members.help.resource.membersAdmin",
  "task.read": "members.help.resource.taskRead",
  "task.create": "members.help.resource.taskCreate",
  "task.update": "members.help.resource.taskUpdate",
  "task.delete": "members.help.resource.taskDelete",
  "action_item.read": "members.help.resource.actionItemRead",
  "action_item.create": "members.help.resource.actionItemCreate",
  "action_item.update": "members.help.resource.actionItemUpdate",
  "action_item.delete": "members.help.resource.actionItemDelete",
  "schedule.read": "members.help.resource.scheduleRead",
  "schedule.create": "members.help.resource.scheduleCreate",
  "schedule.update": "members.help.resource.scheduleUpdate",
  "schedule.delete": "members.help.resource.scheduleDelete",
  "task_assignment.update": "members.help.resource.taskAssignment",
  "schedule_assignment.update": "members.help.resource.scheduleAssignment",
  "occurrence.update": "members.help.resource.occurrenceUpdate",
}

export default function MemberRoleTable({ roles, capabilities }: Props) {
  const i18n = useI18n()

  return (
    <div className={styles.scroll}>
      <table className={styles.table}>
        <thead>
          <tr>
            <th scope="col" className="text-label">{i18n("members.help.action")}</th>
            {roles.map((role) => <th key={role.id} scope="col" className="text-label">{i18n(`members.role.${role.id}` as MessageKey)}</th>)}
          </tr>
        </thead>
        <tbody>
          {capabilities.map((capability) => (
            <tr key={capability.id}>
              <th scope="row" className="text-body">{i18n(ACTION_LABELS[capability.id] ?? "members.help.action")}</th>
              {roles.map((role) => {
                const allowed = capability[role.id as keyof MemberRoleCapability]
                return (
                  <td key={role.id} className={styles.cell} aria-label={`${i18n(`members.role.${role.id}` as MessageKey)}: ${i18n(allowed ? "members.help.allow" : "members.help.deny")}`}>
                    <span aria-hidden="true">{allowed ? "✓" : "—"}</span>
                  </td>
                )
              })}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
