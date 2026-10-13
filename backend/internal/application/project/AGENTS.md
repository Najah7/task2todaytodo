# Project guide

Follow the [application guide](../AGENTS.md). Routes are registered in [cmd/api/main.go](../../../cmd/api/main.go).

## Features

- Create, list, edit, change status, restore, and delete Projects; expose revisions and progress.
- Free users can keep at most three owned Projects that are not deleted or done. Create, restore, explicit reopen, and automatic child-driven reopen check this limit transactionally; existing over-limit Projects remain editable.
- List members and invitations, assign roles, invite by email, resend/revoke invitations, and accept an invitation. Owner, admin, editor, and viewer capabilities come from the role catalog and Project policy.
- Progress combines Task/ActionItem facts and persisted Schedule counts. Invitation acceptance adds membership and consumes the token atomically. A new editor/admin recipient can finish registration first, keep the invitation pending, subscribe to Team, then accept the same invitation.

## Endpoints

- `GET /api/projects`, `/api/projects/options`, `/api/projects/{id}`, `/api/projects/{id}/revisions`, `/api/projects/{id}/members`, `/api/roles/capabilities`.
- `POST /api/projects`, `/api/projects/{id}:start`, `/api/projects/{id}:hold`, `/api/projects/{id}:wait`, `/api/projects/{id}:complete`, `/api/projects/{id}:reopen`, `/api/projects/{id}/restore`.
- `PATCH /api/projects/{id}`, `/api/projects/{id}/status`; `DELETE /api/projects/{id}`.
- `PUT`, `PATCH`, `DELETE /api/projects/{id}/members/{user_id}`; `POST /api/projects/{id}/invitations`, `/api/projects/{id}/invitations/{invitation_id}/resend`; `DELETE /api/projects/{id}/invitations/{invitation_id}`.
- `POST /api/invitations/preview`, `/api/invitations/accept`. Project-related Task and Schedule routes are documented in their owning guides.

## Key files

- [domain/project.go](domain/project.go), [domain/progress.go](domain/progress.go): Project state and progress policy.
- [usecase/member_roles.go](usecase/member_roles.go), [usecase/accept_invitation.go](usecase/accept_invitation.go): role capabilities and transactional acceptance.
- [usecase/progress.go](usecase/progress.go), [repository/progress.go](repository/progress.go): Project-owned progress projection.
- [repository/invitations.go](repository/invitations.go), [application/project_uow.go](../project_uow.go): invitation persistence and cross-context transaction wiring.
- [repository/write_access.go](repository/write_access.go), [usecase/team_access.go](usecase/team_access.go): actual membership and effective Team access enforcement for shared writes.

## Change checks

- Progress reads Task status, nondeleted ActionItem counts, and persisted Schedule counts through the Project repository; apply shared Task percentage policy without importing other contexts' repositories.
- Member removal reassigns Task and Schedule work through transaction-bound commands before deleting membership. Project deletion changes only the parent row.
- Keep membership insertion and invitation consumption atomic, and recheck inviter authorization at acceptance. For a new editor/admin recipient who lacks Team, commit account setup without consuming the invitation, then let the same token be accepted after Team becomes active. Owner Team access is required for accepting any role; role changes affect the [Members UI](../../../../frontend/src/features/project/AGENTS.md).
- Shared Project writes require effective Team access for both actor and owner. Pending invitations do not count as members; losing Team access blocks writes while preserving memberships. Project list/get and member-roster capabilities are projected through the transaction-bound access reader so stale ACL values do not expose edit, invite, or role controls. Owner may remove members as an escape path; viewer invitations remain acceptable by Free/Pro recipients, while editor/admin acceptance requires recipient Team access.

## Verification

- [Project integration tests](../../../tests/integration/project/), especially invitation acceptance, sharing, cross-context flows, and member invitation cases.
