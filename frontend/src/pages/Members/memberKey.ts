import type { RestProjectMemberResponse } from "~/api/generated/projects"

export function projectMemberKey(member: Pick<RestProjectMemberResponse, "user_id" | "invitation_id" | "project_id" | "email">) {
  return member.user_id || member.invitation_id || `${member.project_id}:${member.email || "unknown"}`
}

export function findProjectMember(items: RestProjectMemberResponse[] | undefined, key: string) {
  return items?.find((member) => projectMemberKey(member) === key)
}
