import { describe, expect, it } from "vitest"
import type { RestProjectMemberResponse } from "~/api/generated/projects"
import { findProjectMember, projectMemberKey } from "./memberKey"

describe("project member action targets", () => {
  it("keeps invitation keys distinct when pending rows have empty user IDs", () => {
    const first: RestProjectMemberResponse = { user_id: "", invitation_id: "invite-1", project_id: "project-1", email: "first@example.test" }
    const second: RestProjectMemberResponse = { user_id: "", invitation_id: "invite-2", project_id: "project-1", email: "second@example.test" }

    expect(projectMemberKey(first)).toBe("invite-1")
    expect(projectMemberKey(second)).toBe("invite-2")
    expect(findProjectMember([first, second], projectMemberKey(second))).toBe(second)
  })
})
