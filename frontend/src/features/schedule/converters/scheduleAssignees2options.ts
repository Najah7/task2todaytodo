import type { RestUserResponse } from "~/api/generated/auth"
import type { RestScheduleAssigneeResponse } from "~/api/generated/schedules"
import type { ScheduleAssigneeOption } from "~/features/schedule/providers/ScheduleAPIDataProvider/context"

export function scheduleAssignees2options(assignees: RestScheduleAssigneeResponse[], currentUser: RestUserResponse | undefined, ownerLabel: string): ScheduleAssigneeOption[] {
  return assignees.flatMap((assignee) => {
    if (!assignee.id) return []
    return [{
      id: assignee.id,
      displayName: [assignee.first_name, assignee.last_name].filter(Boolean).join(" ")
        || assignee.email
        || (currentUser?.user_id === assignee.id ? currentUser.user_name : undefined)
        || (assignee.is_owner ? ownerLabel : assignee.id),
    }]
  })
}
