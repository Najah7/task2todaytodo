# Auth guide

Follow the [application guide](../AGENTS.md). Routes are registered in [cmd/api/main.go](../../../cmd/api/main.go).

## Features

- Register and authenticate users; verify email ownership; issue and revoke personal access tokens; reset or change passwords.
- Manage the current user's name, timezone, Active-time settings, and minimum gap. Serve role, permission, and timezone catalogs.
- Registration stores a password hash and sends a one-use email link. Confirmation checks the link and password, marks the email verified, and issues a personal access token in one transaction. Password reset uses a separate auth-link purpose.
- Project invitation acceptance uses Auth's recipient operation inside the Project transaction. Existing verified users authenticate as the invited account; new or unverified recipients become verified on acceptance.

## Endpoints

- `POST /api/signup`, `/api/login`, `/api/email-verification/resend`, `/api/email-verification/confirm`, `/api/password-reset/request`, `/api/password-reset/confirm`.
- `DELETE /api/personal-access-token:revoke`.
- `GET /api/users/me`, `/api/roles`, `/api/permissions`, `/api/timezones`.
- `PATCH /api/users/me`, `/api/users/me/timezone`, `/api/users/me/active-time`, `/api/users/me/minimum-gap`, `/api/users/me/password`.
- Related Project-owned flow: `POST /api/invitations/accept`.

## Key files

- [domain/password.go](domain/password.go): validates input, hashes with Argon2id, and verifies stored hashes.
- [usecase/email_auth.go](usecase/email_auth.go): shared auth-link generation, delivery, and purpose handling.
- [usecase/confirm_email_verification.go](usecase/confirm_email_verification.go), [usecase/confirm_password_reset.go](usecase/confirm_password_reset.go): consume links and update account state.
- [usecase/accept_invitation_recipient.go](usecase/accept_invitation_recipient.go): invitation recipient creation/authentication.
- [repository/email_auth.go](repository/email_auth.go): persisted users, links, and delivery attempts.

## Change checks

- Preserve one-use, expiry, purpose, and transaction checks for email links; never persist plaintext passwords or log raw links/access tokens. Log authentication rejections with a safe reason.
- Keep Project invitation acceptance atomic with membership creation; verify behavior for unregistered, unverified, verified, and wrong-account recipients.
- API or email-flow changes also affect [frontend Auth](../../../../frontend/src/features/auth/AGENTS.md), [Project](../project/AGENTS.md), and generated clients.

## Verification

- [Email-auth integration tests](../../../tests/integration/auth/email_auth_integration_test.go); nearby `domain/*_test.go` and `usecase/*_test.go` cover validation and orchestration.
