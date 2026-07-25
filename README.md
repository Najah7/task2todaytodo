# Task2TodayToDo

Task2TodayToDo turns organized tasks into a daily execution plan. Product focus:

- Project, task, TodoItem, and TaskSchedule management
- Daily TodoList generation
- Timeline planning around fixed TaskSchedules
- Google Calendar integration

## Repository

```text
AGENT.md       Root development guide
backend/       Go API, PostgreSQL schema, Swagger
docs/          Project reference notes
frontend/      React client
```

Read [backend/AGENT.md](backend/AGENT.md) before backend work. Read
[frontend/README.md](frontend/README.md) before frontend work.

## Backend quick start

Prerequisites: Docker, Go, Air.

```bash
cd backend
cp .env.example .env # first setup only
make env-up
make migrate-up
air
```

API docs: <http://localhost:8080/swagger/index.html>

Useful backend commands:

```bash
make go-fmt
make test
make build
make swagger-gen
```
