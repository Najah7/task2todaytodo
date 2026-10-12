import { useEffect, useMemo, useState } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { Link, useSearchParams } from "react-router"
import {
  getProjectsIdMembers,
  patchProjectsIdMembersUserId,
  deleteProjectsIdMembersUserId,
  postProjectsIdInvitations,
  postProjectsIdInvitationsInvitationIdResend,
  deleteProjectsIdInvitationsInvitationId,
} from "~/api/generated/projects"
import type { RestProjectMemberListResponse, RestProjectMemberResponse, RestProjectResponse } from "~/api/generated/projects"
import { ApiError } from "~/api/http"
import { useI18n, useLanguage, type MessageKey } from "~/features/i18n/hooks"
import InviteMemberDialog, { type InviteRole } from "~/features/project/components/InviteMemberDialog"
import type { InviteMemberValues } from "~/features/project/components/InviteMemberDialog/schema"
import MemberProjectSidebar from "~/features/project/components/MemberProjectSidebar"
import type { MemberProjectSummary } from "~/features/project/components/MemberProjectSidebar"
import MemberRoster from "~/features/project/components/MemberRoster"
import type { MemberRosterItem } from "~/features/project/components/MemberRoster"
import { fetchActiveProjects } from "~/features/project/providers/ProjectAPIDataProvider/fetchActiveProjects"
import { findProjectMember, projectMemberKey } from "./memberKey"
import ConfirmationDialog from "~/features/shared/components/ConfirmationDialog"
import PageHeading from "~/features/shared/components/PageHeading"
import { notify } from "~/features/shared/notification"
import controls from "~/styles/controls.module.css"
import styles from "./index.module.css"

const PAGE_SIZE = 20

type MemberApiItem = RestProjectMemberResponse
type MemberApiPage = RestProjectMemberListResponse
type PendingAction =
  | { kind: "remove"; member: MemberApiItem }
  | { kind: "revoke"; member: MemberApiItem }
  | { kind: "resend"; member: MemberApiItem }
  | { kind: "role"; member: MemberApiItem; roleId: string }

function displayName(member: MemberApiItem) {
  return [member.first_name, member.last_name].filter(Boolean).join(" ")
}

function invitationDate(value: number | null | undefined, language: string) {
  if (!value) return undefined
  return new Intl.DateTimeFormat(language, { dateStyle: "medium" }).format(new Date(value * 1000))
}

function toProjectSummary(project: RestProjectResponse & { member_count?: number; pending_invitation_count?: number }): MemberProjectSummary {
  return {
    id: project.id,
    title: project.title,
    memberCount: project.member_count ?? 0,
    pendingInvitationCount: project.pending_invitation_count ?? 0,
  }
}

export default function MembersPage() {
  const i18n = useI18n()
  const { language } = useLanguage()
  const queryClient = useQueryClient()
  const [searchParams, setSearchParams] = useSearchParams()
  const [pageTokens, setPageTokens] = useState<string[]>([])
  const [inviteOpen, setInviteOpen] = useState(false)
  const [inviteError, setInviteError] = useState<MessageKey | null>(null)
  const [inviteBusy, setInviteBusy] = useState(false)
  const [action, setAction] = useState<PendingAction | null>(null)
  const [actionBusy, setActionBusy] = useState(false)

  const projectsQuery = useQuery({
    queryKey: ["projects", "active-options"],
    queryFn: ({ signal }) => fetchActiveProjects(signal),
    staleTime: 60_000,
    retry: false,
  })
  const projects = useMemo(() => (projectsQuery.data ?? []).map(toProjectSummary), [projectsQuery.data])
  const selectedProjectId = searchParams.get("projectId") ?? ""
  const selectedProject = projects.find((project) => project.id === selectedProjectId) ?? projects[0]
  const currentPageToken = pageTokens.at(-1) ?? ""
  const membersQuery = useQuery({
    queryKey: ["projectMembers", selectedProject?.id, currentPageToken],
    queryFn: () => getProjectsIdMembers(selectedProject!.id, { page_size: PAGE_SIZE, page_token: currentPageToken || undefined }),
    enabled: Boolean(selectedProject),
    retry: false,
  })
  const page = membersQuery.data as MemberApiPage | undefined
  const roles: InviteRole[] = page?.assignable_roles?.filter((role) => role.assignable && role.id).map((role) => ({ id: role.id! })) ?? []

  useEffect(() => {
    setPageTokens([])
  }, [selectedProject?.id])

  useEffect(() => {
    if (!selectedProject) return
    if (searchParams.get("projectId") === selectedProject.id) return
    const next = new URLSearchParams(searchParams)
    next.set("projectId", selectedProject.id)
    setSearchParams(next, { replace: true })
  }, [searchParams, selectedProject, setSearchParams])

  function selectProject(projectId: string) {
    const next = new URLSearchParams(searchParams)
    next.set("projectId", projectId)
    setSearchParams(next)
    void queryClient.invalidateQueries({ queryKey: ["projectMembers"] })
  }

  const roster: MemberRosterItem[] = (page?.items ?? []).map((member) => {
    const name = displayName(member)
    const expires = invitationDate(member.expires_at, language)
    return {
      key: projectMemberKey(member),
      name,
      email: member.email ?? "",
      roleId: member.role_id ?? "viewer",
      status: member.status ?? "accepted",
      canEditRole: member.can_edit_role ?? false,
      canRemove: member.can_remove ?? false,
      expirationLabel: expires ? i18n("members.inviteExpired", { date: expires }) : undefined,
    }
  })

  async function submitInvite(values: InviteMemberValues) {
    if (!selectedProject || inviteBusy) return
    setInviteBusy(true)
    setInviteError(null)
    try {
      await postProjectsIdInvitations(selectedProject.id, { email: values.email, role_id: values.roleId as "admin" | "editor" | "viewer" })
      setInviteOpen(false)
      notify.success(i18n("members.inviteDialog.success"))
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["projectMembers", selectedProject.id] }),
        queryClient.invalidateQueries({ queryKey: ["projects", "active-options"] }),
        queryClient.invalidateQueries({ queryKey: ["/api/projects"] }),
      ])
      setPageTokens([])
    } catch (error) {
      setInviteError(error instanceof ApiError && error.status === 403 ? "members.error.forbidden" : "members.inviteDialog.error")
    } finally {
      setInviteBusy(false)
    }
  }

  async function confirmAction() {
    if (!action || !selectedProject || actionBusy) return
    setActionBusy(true)
    try {
      if (action.kind === "remove") {
        await deleteProjectsIdMembersUserId(selectedProject.id, action.member.user_id ?? "")
        notify.success(i18n("members.toast.removed"))
      } else if (action.kind === "revoke") {
        await deleteProjectsIdInvitationsInvitationId(selectedProject.id, action.member.invitation_id ?? "")
        notify.success(i18n("members.toast.revoked"))
      } else if (action.kind === "resend") {
        await postProjectsIdInvitationsInvitationIdResend(selectedProject.id, action.member.invitation_id ?? "")
        notify.success(i18n("members.toast.resent"))
      } else {
        await patchProjectsIdMembersUserId(selectedProject.id, action.member.user_id ?? "", { role_id: action.roleId })
        notify.success(i18n("members.toast.roleUpdated"))
      }
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["projectMembers", selectedProject.id] }),
        queryClient.invalidateQueries({ queryKey: ["projects", "active-options"] }),
        queryClient.invalidateQueries({ queryKey: ["/api/projects"] }),
      ])
      setPageTokens([])
      setAction(null)
    } catch (error) {
      notify.error(i18n(error instanceof ApiError && error.status === 403 ? "members.error.forbidden" : "members.error.generic"))
      await membersQuery.refetch()
    } finally {
      setActionBusy(false)
    }
  }

  const actionTarget = action?.member.email ?? ""
  const actionName = (action ? displayName(action.member) : "") || actionTarget
  const actionTitle = action?.kind === "remove"
    ? i18n("members.confirm.removeTitle")
    : action?.kind === "revoke"
      ? i18n("members.confirm.revokeTitle")
      : action?.kind === "resend"
        ? i18n("members.confirm.resendTitle")
        : action?.kind === "role"
          ? i18n("members.confirm.roleTitle")
          : ""
  const actionDescription = action?.kind === "remove"
    ? i18n("members.confirm.removeDescription", { name: actionName })
    : action?.kind === "revoke"
      ? i18n("members.confirm.revokeDescription", { email: actionTarget })
      : action?.kind === "resend"
        ? i18n("members.confirm.resendDescription", { email: actionTarget })
        : action?.kind === "role"
          ? i18n("members.confirm.roleDescription", { name: actionName, role: i18n(`members.role.${action.roleId}` as MessageKey) })
          : ""
  const actionConfirm = action?.kind === "remove"
    ? i18n("members.confirm.remove")
    : action?.kind === "revoke"
      ? i18n("members.confirm.revoke")
      : action?.kind === "resend"
        ? i18n("members.confirm.resend")
        : action?.kind === "role"
          ? i18n("members.confirm.roleTitle")
          : ""

  return (
    <section className={styles.page} aria-labelledby="members-heading">
      <header className={styles.pageHeader}>
        <PageHeading id="members-heading" messageKey="page.members.title" descriptionKey="members.description" />
        <div className={styles.headerActions}>
          <Link className={`${controls.button} ${controls.neutralButton} ${controls.focusRing} text-button-small`} to={`/members/roles${selectedProject ? `?projectId=${encodeURIComponent(selectedProject.id)}` : ""}`}>
            {i18n("members.roleHelp")}
          </Link>
          {selectedProject && page?.can_invite && <button className={`${controls.button} ${controls.primaryButton} ${controls.focusRing} text-button`} type="button" onClick={() => setInviteOpen(true)}>{i18n("members.invite")}</button>}
        </div>
      </header>
      {projectsQuery.isPending ? <p className={`${styles.state} text-body`}>{i18n("members.loading")}</p> : projectsQuery.isError ? (
        <div className={styles.state} role="alert"><p className="text-body">{i18n("members.loadError")}</p><button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} type="button" onClick={() => void projectsQuery.refetch()}>{i18n("members.retry")}</button></div>
      ) : projects.length === 0 ? <p className={`${styles.state} text-body`}>{i18n("members.noProjects")}</p> : (
        <div className={styles.content}>
          <MemberProjectSidebar projects={projects} selectedId={selectedProject?.id ?? ""} onSelect={selectProject} />
          <section className={styles.rosterArea} aria-label={selectedProject?.title}>
            {membersQuery.isPending ? <p className={`${styles.state} text-body`}>{i18n("members.loading")}</p> : membersQuery.isError ? (
              <div className={`${controls.card} ${styles.state}`} role="alert"><p className="text-body">{i18n("members.loadError")}</p><button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} type="button" onClick={() => void membersQuery.refetch()}>{i18n("members.retry")}</button></div>
            ) : page && selectedProject ? (
              <>
                <section className={`${controls.card} ${styles.rosterCard}`} aria-label={selectedProject.title}>
                  <header className={styles.rosterHeader}>
                    <div><h2 className="text-card-title">{selectedProject.title}</h2><p className="text-caption">{i18n("members.memberCount", { count: page.member_count ?? 0 })}{(page.pending_invitation_count ?? 0) > 0 ? ` · ${i18n("members.pendingCount", { count: page.pending_invitation_count ?? 0 })}` : ""}</p></div>
                  </header>
                  {roster.length === 0 ? <p className={`${styles.empty} text-body`}>{i18n("members.empty")}</p> : (
                    <MemberRoster
                      items={roster}
                      roles={roles}
                      canManageInvitations={page.can_invite ?? false}
                      onRoleChange={(row, roleId) => {
                        const member = findProjectMember(page.items, row.key)
                        if (member) setAction({ kind: "role", member, roleId })
                      }}
                      onRemove={(row) => { const member = findProjectMember(page.items, row.key); if (member) setAction({ kind: "remove", member }) }}
                      onResend={(row) => { const member = findProjectMember(page.items, row.key); if (member) setAction({ kind: "resend", member }) }}
                      onRevoke={(row) => { const member = findProjectMember(page.items, row.key); if (member) setAction({ kind: "revoke", member }) }}
                    />
                  )}
                  <nav className={styles.pagination} aria-label={i18n("members.pagination")}>
                    <button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} type="button" disabled={pageTokens.length === 0 || membersQuery.isFetching} onClick={() => setPageTokens((tokens) => tokens.slice(0, -1))}>{i18n("members.previous")}</button>
                    <span className="text-caption" aria-current="page">{i18n("members.pageLabel", { page: pageTokens.length + 1 })}</span>
                    <button className={`${controls.button} ${controls.outlineButton} ${controls.focusRing} text-button-small`} type="button" disabled={!page.next_page_token || membersQuery.isFetching} onClick={() => { if (page.next_page_token) setPageTokens((tokens) => [...tokens, page.next_page_token!]) }}>{i18n("members.next")}</button>
                  </nav>
                </section>
              </>
            ) : null}
          </section>
        </div>
      )}
      <InviteMemberDialog open={inviteOpen} projectName={selectedProject?.title ?? ""} roles={roles} busy={inviteBusy} error={inviteError} onClose={() => setInviteOpen(false)} onSubmit={submitInvite} />
      <ConfirmationDialog open={Boolean(action)} title={actionTitle} description={actionDescription} confirmLabel={actionConfirm} cancelLabel={i18n("members.confirm.cancel")} destructive={action?.kind === "remove" || action?.kind === "revoke"} busy={actionBusy} onCancel={() => setAction(null)} onConfirm={() => void confirmAction()} />
    </section>
  )
}
