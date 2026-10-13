# Project feature guide

Follow the [feature guide](../AGENTS.md). Paths are defined in [router.tsx](../../router.tsx); backend behavior is in the [Project guide](../../../../backend/internal/application/project/AGENTS.md).

## Features and pages

- `/projects`, `/projects/new`, `/projects/:id/edit`: list, create, edit, change status, and restore Projects.
- Free users may own at most 3 active personal Projects. Keep existing over-cap Projects available for ordinary edits and completion; explain create/restore/reopen denials with the Free limit message.
- `/members`: select a Project, inspect members and invitations, invite, change roles, resend/revoke invitations, or remove a member. `/members/roles` explains capabilities.
- `/invitations`: preview and explicitly accept an email invitation, with Auth login or password setup as needed.

## Key files

- [ProjectAPIDataProvider](providers/ProjectAPIDataProvider/index.tsx), [ProjectActionProvider](providers/ProjectActionProvider/index.tsx), [ProjectNavigationProvider](providers/ProjectNavigationProvider/index.tsx): list/edit data, mutations, and URL state.
- [useProjectEditForm.ts](hooks/useProjectEditForm.ts), [ProjectForm](components/ProjectForm/index.tsx): edit draft and shared form.
- [pages/Members](../../pages/Members/index.tsx), [MemberRoster](components/MemberRoster/index.tsx), [InviteMemberDialog](components/InviteMemberDialog/index.tsx): membership UI.
- [pages/InvitationAcceptance](../../pages/InvitationAcceptance/index.tsx), [InvitationAcceptForm](components/InvitationAcceptForm/index.tsx): invitation acceptance.

## Change checks

- Preserve Project revision baseline across background refetches; conflict reload must not discard a dirty draft without confirmation.
- Render assignable roles and row actions from server capabilities. Pending/expired invitations have resend/revoke actions; accepted members have role/remove actions. Team loss removes invite and role-change controls; the owner can still remove members to return the Project to personal use, and the owner row remains non-removable.
- Invitation preview does not accept the invitation. Acceptance must use the invited account; preserve the wrong-account and new-user password flows.
- When an unregistered editor/admin recipient completes account setup but needs Team, keep the invitation token in tab-scoped session storage, route to Billing, and return to the same invitation after the backend reports effective Team access. Do not ask a viewer to upgrade when the Project owner needs to restore Team.

## Verification

- [Projects E2E](../../../e2e/projects.spec.ts), [Members E2E](../../../e2e/members.spec.ts), and colocated form/roster tests.
