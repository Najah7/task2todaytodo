import { afterEach, expect, test } from "vitest"
import { clearPendingTeamInvitation, invitationPath, pendingTeamInvitation, rememberTeamInvitation } from "./teamInvitationReturn"

afterEach(() => sessionStorage.clear())

test("keeps the invite token in this tab and builds a same-invite return path", () => {
  rememberTeamInvitation("token / value")
  expect(pendingTeamInvitation()).toBe("token / value")
  expect(invitationPath(pendingTeamInvitation()!)).toBe("/invitations#token=token%20%2F%20value")

  clearPendingTeamInvitation()
  expect(pendingTeamInvitation()).toBeNull()
})
