# Project guide

Follow the [application guide](../AGENTS.md). Routes are registered in [cmd/api/main.go](../../../cmd/api/main.go).

## Features

- Create, list, edit, change status, restore, and delete Projects; expose revisions and progress.
- List members and invitations, assign roles, invite by email, resend/revoke invitations, and accept an invitation. Owner, admin, editor, and viewer capabilities come from the role catalog and Project policy.
- Progress combines Task/ActionItem facts and persisted Schedule counts. Invitation acceptance adds membership and consumes the token in one transaction.

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

## Change checks

- Progress reads Task status, nondeleted ActionItem counts, and persisted Schedule counts through the Project repository; apply shared Task percentage policy without importing other contexts' repositories.
- Member removal reassigns Task and Schedule work through transaction-bound commands before deleting membership. Project deletion changes only the parent row.
- Keep invitation token consumption, recipient creation/verification, and membership insertion atomic; recheck inviter authorization at acceptance. Role changes affect the [Members UI](../../../../frontend/src/features/project/AGENTS.md).

## Verification

- [Project integration tests](../../../tests/integration/project/), especially invitation acceptance, sharing, cross-context flows, and member invitation cases.
