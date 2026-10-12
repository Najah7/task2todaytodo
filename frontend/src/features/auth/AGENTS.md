# Auth feature guide

Follow the [feature guide](../AGENTS.md). Paths are defined in [router.tsx](../../router.tsx); backend behavior is in the [Auth guide](../../../../backend/internal/application/auth/AGENTS.md).

## Features and pages

- `/login`: authenticate; `/signup`: enter account details and request email verification.
- `/verify-email`: confirm the link and password; `/request-password-reset` and `/reset-password`: request and use a reset link.
- `/invitations` combines Auth sign-in/password setup with the [Project invitation UI](../project/AGENTS.md).

## Key files

- [components/SignupForm](components/SignupForm/index.tsx), [components/VerifyEmailForm](components/VerifyEmailForm/index.tsx): registration and verification forms.
- [components/ResetPasswordForm](components/ResetPasswordForm/index.tsx), [hooks/useLogin.ts](hooks/useLogin.ts): recovery and login actions.
- [hooks/useEmailActionToken.ts](hooks/useEmailActionToken.ts): reads email-link token from the URL fragment.
- [lib/localStorage.ts](lib/localStorage.ts), [errors.ts](errors.ts): personal access token storage and API error messages.

## Change checks

- Keep link confirmation behind an explicit user submit; merely opening a verification or invitation URL must not consume a token.
- Server validation remains authoritative. After successful login/verification/reset/invitation acceptance, coordinate token storage, Query cache, and navigation with the Pages.
- Forms provide immediate field errors; email delivery/expiry and wrong-account cases need their own UI states. Do not expose email-link tokens in logs.

## Verification

- [Email Auth E2E](../../../e2e/email-auth.spec.ts), [signup E2E](../../../e2e/signup.spec.ts), [login E2E](../../../e2e/login.spec.ts), and colocated form/schema tests.
