# End-to-end tests

Playwright exercises important user journeys against the real Go API and PostgreSQL. API responses are not mocked. Detailed validation and simulated failures belong in unit tests beside their implementation.

## Run

Prerequisites: Node.js and pnpm, a running Docker daemon with Docker Compose v2, and Playwright Chromium.

From `frontend/`:

```bash
pnpm install
pnpm exec playwright install chromium --only-shell
pnpm test:e2e
pnpm test:e2e --workers=2
pnpm test:e2e login.spec.ts
```

The first run downloads Docker images and Go dependencies. Later builds reuse Docker's build cache. Playwright arguments are forwarded by the runner. Use `pnpm test:e2e` rather than invoking Playwright directly so the test environment is always initialized.

When deploying the SPA, serve its HTML entry document with `Cache-Control: no-store` so verification and reset landing pages are not cached by browsers or intermediaries. The entry document sets `Referrer-Policy` to `no-referrer`, and email-link credentials stay in URL fragments so browsers do not send them in HTTP requests.

## Lifecycle and isolation

`run.mjs` creates a unique Compose project, starts a temporary database and Mailpit, applies the existing backend migrations, builds the backend, and waits for its health check. Playwright then starts a separate Vite server whose API proxy targets that backend. API, frontend, and Mailpit ports are allocated dynamically; existing development servers are not reused.

Each test creates its own users with unique email addresses through the real API. Tests do not depend on execution order or shared seed data. Authentication assertions check that the saved token can access `/api/users/me`.

After success, failure, or a handled interrupt, the runner removes its containers, network, volumes, and API image. Database data lives in a temporary filesystem. Build caches remain available for subsequent runs. A forced process kill cannot run cleanup; use the project name printed at startup to clean up manually:

```bash
E2E_PAGE_TOKEN_KEY=unused docker compose -p <printed-project-name> -f e2e/compose.yml down --volumes --rmi local
```

On failure, the runner prints service logs before cleanup. Playwright retains traces in `test-results/`.

## Coverage

- Email signup, Mailpit verification, explicit confirmation, and normal login.
- Unverified login handling and resend guidance; wrong credentials remain generic.
- Mailpit password-reset flow, explicit confirmation, and normal login with the new password.
- Generic reset guidance for eligible, ineligible, and unknown email addresses.
- Language and display preferences, including persistence across navigation and reloads.
