# Task2TodayToDo

[日本語](README.ja.md)

Task2TodayToDo turns tasks and fixed schedules into a doable plan for today. It considers available time, Project allocations, and work priority, so you spend less time deciding what fits around meetings. You can adjust the generated Todo; later changes to source work do not automatically rebuild it.

## Run locally

You need Docker with Compose, Node.js, and pnpm 10.5.2. Run these commands from the repository root in two terminals.

**Terminal 1 — backend**

```bash
cd backend
test -f .env || cp .env.example .env
```

If `PAGE_TOKEN_KEY` still has the sample value, run `openssl rand -base64 32` and put its output in `backend/.env`. Then start PostgreSQL and the local email inbox, wait until the database is healthy, and start the API:

```bash
make env-up
until [ "$(docker inspect --format '{{.State.Health.Status}}' task2todaytodo-db)" = healthy ]; do sleep 1; done
make migrate-up
make dev
```

**Terminal 2 — frontend**

```bash
cd frontend
test -f .env || cp .env.example .env
pnpm install --frozen-lockfile
pnpm dev
```

Open <http://localhost:5173>, sign up, and find the verification email at <http://localhost:8025>. After verifying and logging in, create a Task or Schedule and open **Today** to see the generated plan.

Stripe billing is optional for local use. The app runs with the Stripe fields in `backend/.env` empty; billing needs Sandbox credentials and webhook forwarding.
