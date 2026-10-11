import type { RestUserResponse } from "~/api/generated/auth"
import type { RestProjectMemberResponse, RestProjectResponse } from "~/api/generated/projects"
import type { ScheduleAssigneeOption } from "~/features/schedule/providers/ScheduleAPIDataProvider/context"

export function projectMembers2assigneeOptions(
  members: RestProjectMemberResponse[],
  project: RestProjectResponse | undefined,
  currentUser: RestUserResponse | undefined,
  ownerLabel: string,
  projectOwnerId?: string,
  currentUserLabel = ownerLabel,
): ScheduleAssigneeOption[] {
  const options = members.filter((member) => member.user_id).map((member) => ({
    id: member.user_id!,
    displayName: [member.first_name, member.last_name].filter(Boolean).join(" ")
      || member.email
      || (currentUser?.user_id === member.user_id ? currentUser?.user_name : undefined)
      || member.user_id!,
  }))
  const ownerId = project?.user_id || projectOwnerId
  if (ownerId && !options.some((option) => option.id === ownerId)) {
    const displayName = ownerId === currentUser?.user_id ? currentUser?.user_name || ownerLabel : ownerLabel
    options.unshift({ id: ownerId, displayName })
  }
  if (currentUser?.user_id && !options.some((option) => option.id === currentUser.user_id)) {
    options.unshift({ id: currentUser.user_id, displayName: currentUser.user_name || currentUserLabel })
  }
  return options
}
