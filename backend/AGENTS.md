# Backend guide

## Map

```text
cmd/api/                                  API entry and dependency wiring
db/migrations/                            PostgreSQL schema
db/queries/                               sqlc SQL source
db/sqlc/                                  Generated sqlc package
docs/                                     Generated Swagger
internal/application/                    DB config, use case and UOW wiring
internal/application/<context>/domain/   VO, entity, domain tests
internal/application/<context>/dao/      Primitive-field read models
internal/application/<context>/usecase/  Single-operation use cases, ports, tests
internal/application/<context>/repository/ DB adapter and sqlc mapping
internal/application/shared/             Cross-context application contracts
internal/application/shared/pagination/  Transport-independent List page policy
internal/config/                          Process environment loading and validation
internal/logging/                         Structured logger contract and slog implementation
internal/telemetry/                       OpenTelemetry SDK setup, propagation and trace export
internal/application/shared/calendar/    Shared calendar and wall-time helpers
internal/application/shared/recurrence/  Shared recurrence values and occurrence rules
internal/port/rest/                      HTTP handlers, response and error mapping
internal/port/rest/middleware/           HTTP middleware
internal/port/rest/pagination/           Query parsing and encrypted page tokens
internal/port/rest/fieldmask/            Read response field masks
internal/port/adapter/                   External adapters, including ULID and UUID
```

`<context>` means `auth`, `project`, `task`, `schedule`, or `tag`. Their HTTP
handlers all live in `internal/port/rest`.

## Commands

```bash
$ make help
Available commands:
  make go-fmt                  Format Go source files.
  make test                    Run Go tests.
  make test-integration        Start isolated PostgreSQL and run integration tests.
  make build                   Build the API binary.
  make run                     Run the app service with Docker Compose.
  make dev                     Run the API with Air live reload in Docker.
  make env-up                  Start the database service.
  make env-down                Stop the database service.
  make env-cleanup             Remove the database volume after confirmation.
  make cleanup-logs            Remove application logs after confirmation.
  make db-shell                Open a psql shell in the database container.
  make migrate-up              Run database migrations.
  make migrate-down            Roll back database migrations.
  make migrate-create name=... Create a new SQL migration.
  make sqlc-gen                Generate sqlc repository code.
  make swagger-gen             Generate Swagger documentation.
  make swagger-fmt             Format Swagger annotations.
```

## Layer rule

```text
cmd/api -> port/rest + application wiring
port/rest and middleware -> application/<context>/usecase, dao/domain
application/<context>/usecase -> own dao/domain + application/shared
application/<context>/repository -> own usecase ports + dao/domain + sqlc/DB
application root -> usecase + repository impl + UOW + port/adapter wiring
```

The application root composes context packages. See the
[application guide](internal/application/AGENTS.md) for application boundaries,
domain modeling, read/write rules, UOWs, and repository conventions.

## Area guides

Before changing an area below, read its guide:

- [REST handlers, errors, and list APIs](internal/port/rest/AGENTS.md)
- [Database migrations and SQL generation](db/AGENTS.md)
- [Runtime configuration](internal/config/AGENTS.md)
- [Logging across backend layers](internal/logging/AGENTS.md)
- [OpenTelemetry setup and trace export](internal/telemetry/AGENTS.md)
- [Application architecture and rules](internal/application/AGENTS.md)

## Test

- `make test` runs unit tests only. Keep pure database-config and usecase tests
  in their package directories; they must not require PostgreSQL.
- Real PostgreSQL tests live under `tests/integration/{project,task,schedule,tag,rest}`
  and use the `integration` build tag. Share only the guarded pool helper in
  `tests/integration/internal/testdb`.
- `make test-integration` starts a uniquely named PostgreSQL container on a
  dynamically assigned loopback port, applies every `db/migrations/*.up.sql`
  file in order, runs the tagged suite serially, then removes only that
  container. It needs Docker and does not use the development database.
- Direct tagged test runs require `INTEGRATION_DATABASE_URL` pointing to a
  disposable database named `task2todaytodo_integration_*`; tests fail when
  the variable is absent or the database name is outside that allowlist.
- Do not add `t.Parallel` to integration tests until shared database state is
  isolated per test.
- Add regression tests for database mapping, nullable fields, and public error
  mapping bugs.
- See the [application guide](internal/application/AGENTS.md) for
  application-specific unit-test conventions.

## Done

- Req met. Layer rule kept.
- Changed Go formatted.
- `make test` passes when workspace buildable.
- Generated output refreshed after source change.
