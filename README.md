# Task2TodayToDo

[日本語](README.ja.md)

***Don’t live for tomorrow. Give today everything you’ve got.***

task2todaytodo manages Projects, Tasks, ActionItems, and independent Schedules, then creates today's doable workload list.

The core problem is that planning work across a week or month is easy to start but hard to maintain. Interruptions create drift, and manually repairing a long-term calendar quickly becomes too much work. task2todaytodo avoids that burden by letting users manage Projects, Tasks, ActionItems, and Schedules together in a structured source of truth, then generating each user's Todo once for the local day.

The generated Todo selects whole work items that fit around fixed Schedules and active time, respecting project allocations and work priority. Users can add or remove work and reorder the list after generation. They can also explicitly regenerate today's Todo with its current revision; this replaces all daily rows and manual display changes while preserving completion and actual minutes on source records.

# Values

- Task Management: manage Tasks and their ActionItems, including repeatable and one-off work.
- Schedule Management: manage fixed-time Schedules independently or within Projects, including repeatable and one-off work. Future integrations can synchronize them with calendar systems such as Google Calendar.
- Generate Today's Todo: generate a daily workload list once per user-local day, based on eligible work, active time, project allocations, and fixed Schedules. Users can adjust its contents and display order.

## Backend quick start

Prerequisites: Docker, Go, Air.

```bash
cd backend
cp .env.example .env # first setup only
openssl rand -base64 32 # set output as PAGE_TOKEN_KEY in backend/.env
make env-up
make migrate-up
air
```

API docs: <http://localhost:8080/swagger/index.html>

Keep `PAGE_TOKEN_KEY` stable across API instances. Changing it invalidates existing page tokens for up to 24 hours.

Check Useful backend commands:

```bash
make help        # show help
```

### Logs and traces

See the [backend logging and tracing overview](backend/README.md#logs-and-traces)
for a short overview and links to implementation and configuration guidance.

## Frontend quick start

Prerequisites: Node.js and pnpm 10.5.2. From the repository root, in a separate terminal, start the frontend after starting the backend using the instructions above. Vite proxies `/api` to `http://127.0.0.1:8080` by default.

```bash
cd frontend
pnpm install --frozen-lockfile
cp .env.example .env # first setup only
pnpm dev
```

Open <http://localhost:5173>. Set `API_PROXY_TARGET` in `frontend/.env` if the backend uses a different address. See the [frontend README](frontend/README.md) for more commands and details.
