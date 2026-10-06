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

## Lifecycle and isolation

`run.mjs` creates a unique Compose project, starts a temporary database, applies the existing backend migrations, builds the backend, and waits for its health check. Playwright then starts a separate Vite server whose API proxy targets that backend. API and frontend ports are allocated dynamically; existing development servers are not reused.

Each test creates its own users with unique email addresses through the real API. Tests do not depend on execution order or shared seed data. Authentication assertions check that the saved token can access `/api/users/me`.

After success, failure, or a handled interrupt, the runner removes its containers, network, volumes, and API image. Database data lives in a temporary filesystem. Build caches remain available for subsequent runs. A forced process kill cannot run cleanup; use the project name printed at startup to clean up manually:

```bash
E2E_PAGE_TOKEN_KEY=unused docker compose -p <printed-project-name> -f e2e/compose.yml down --volumes --rmi local
```

On failure, the runner prints service logs before cleanup. Playwright retains traces in `test-results/`.

## Coverage

- Signup, automatic login, and a persisted session accepted by the backend.
- Incorrect credentials, duplicate email, and correction of password confirmation.
- Language and display preferences, including persistence across navigation and reloads.
