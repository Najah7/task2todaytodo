const PENDING_TEAM_INVITATION_KEY = "task2todaytodo:pending-team-invitation"

export function rememberTeamInvitation(token: string) {
  sessionStorage.setItem(PENDING_TEAM_INVITATION_KEY, token)
}

export function pendingTeamInvitation() {
  return sessionStorage.getItem(PENDING_TEAM_INVITATION_KEY)
}

export function clearPendingTeamInvitation() {
  sessionStorage.removeItem(PENDING_TEAM_INVITATION_KEY)
}

export function invitationPath(token: string) {
  return "/invitations#token=" + encodeURIComponent(token)
}
