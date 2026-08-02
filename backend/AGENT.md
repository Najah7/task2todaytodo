# Backend guide

## Map

```text
cmd/api/                 API entry
db/migrations/           PostgreSQL schema
db/queries/              sqlc SQL source
db/sqlc/                 Generated sqlc package
docs/                    Generated Swagger
internal/application/    Wiring, config
internal/shared/         Cross-context ID and shared packages
internal/<domain>/domain/ Domain VO, entity, aggregate, domain tests
internal/<domain>/handlers/ HTTP boundary
internal/<domain>/middlewares/ HTTP middleware
internal/<domain>/repository/ DB adapters, sqlc mapping
internal/<domain>/usecase/ Application services, repository ports, usecase tests
internal/<domain>/adapters/ External adapters
```

`<domain>` is `auth` or `task`.

`db/queries` + `sqlc.yml` = sqlc source. `/db/sqlc` = generated.

## Commands

```bash
$ make help
Available commands:
  make go-fmt                  Format Go source files.
  make test                    Run Go tests.
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
handler/middleware -> usecase -> domain
application -> usecase + repository impl wiring
repository impl -> sqlc/DB
```

Domain no HTTP, DB, framework, driver imports. Usecase owns repository ports.
Repository impl imports usecase ports and maps DB records to domain objects.
Application wires concrete repositories into usecases.

`internal/<context>/` owns one bounded context. Keep domain/usecase/repository
inside that context. `internal/domain/shared` is only for cross-context shared
domain concepts.

### Context dependencies

```text
auth, task -> shared
main.go, application.go -> auth, task, shared
```

- Do not import one domain from another: `task -> auth`, `auth -> task`, and
  `shared -> auth/task` are forbidden.
- Any domain may import `shared`, but `shared` must not depend on a domain.
- If a cross-domain reference is unavoidable, define the required interface on
  the importing side and inject the implementation from `main.go` or
  `application.go`.
- These rules exist to prevent circular imports.

### VO

- File prefix `value_`.
- One validated concept.
- Validate in constructor.
- Keep representation work, e.g. hash, here.
- Separate input constructor from persisted-state restore when invariants differ.

### Entity

- File prefix `entity_`.
- Factory create. No external struct literal.
- Own invariant, state change, state query.
- New-state and restored-state factory may differ.
- Model absent state explicit. Never infer state from invalid DB data.
- `NewXXX`: minimal required fields only. Keep create path simple.
- `NewXXXWithDetails`: richer create path when optional/detail fields are provided.
- `NewExistingXXX`: restore persisted state. Accept all stored fields, including timestamps.
- `NewZeroXXX`: explicit absent/invalid return value.

### Aggregate

- File prefix `aggregate_`.
- Use Aggregate for read models that span multiple tables or entity collections.
- JOIN-based queries should return repository-mapped Aggregates, e.g. `ProjectAggregate` for Project + Tasks or `TaskAggregate` for Task + TodoItems + TaskSchedules.
- Keep SQL/JOIN details inside repository implementations. Domain Aggregates describe the composed domain result, not database mechanics.
- Do not force a single-table Entity to carry child collections only because one API response needs them.

### Application Service

- File prefix `service_`.
- Orchestrate domain object load, domain method call, repository save, and transaction boundary.
- Keep repository interfaces in the usecase package, close to the Application Service that needs them.
- No duplicate VO/entity validation.
- No HTTP req/context. Handler extracts transport data, passes args.
- Pre-check helps. DB constraint final authority.
- Prefer entity/VO/aggregate methods for domain behavior.
- Avoid Domain Service by default. Add domain behavior to the domain object when it naturally belongs there.
- If behavior cannot be expressed cleanly as a domain method, implement the orchestration in Application Service.
- Introduce Domain Service only for rare pure-domain behavior that does not belong to any entity/VO and does not need repository/DB access.

### Transaction

- Implement transaction support with the Unit of Work pattern.
- Usecase owns `UnitOfWork` and `Repositories` interfaces.
- Application/infrastructure implements `UnitOfWork` with DB begin, commit, rollback.
- In Go, `UnitOfWork` receives the workflow function, e.g. `uow.Do(ctx, func(ctx, repos) error { ... })`.
- Do not use an event-registration style where operations are queued and executed at the end.
- Prepare one Unit of Work per domain concern/bounded context, e.g. auth UoW and task UoW.
- A UoW exposes only repositories for its own bounded context.
- Repository impl may expose `WithTx(tx)` internally. Do not add `WithTx` to usecase repository ports.
- Application Service decides the transaction boundary by calling `uow.Do(...)`.
- Inside `uow.Do(...)`, use only repositories received from `repos`. Do not call service fields backed by non-transaction repositories.
- Domain never knows transaction, repository, driver, or context-carried DB state.

## Handler and public error

- Handler owns decode, auth/context extract, HTTP status, error map.
- Every error res needs stable, client-safe detail.
- Map known domain and DB constraint errors → stable status, field, detail code.
- Never return raw DB error, connection string, SQL, password, stack trace.
- Unknown failure → safe generic detail. Keep root error for server diagnostics.
- Swagger type names and statuses match real handler behavior.

## Repository and SQL

- Repository impl lives in `internal/<context>/repository`.
- Repository ports live in `internal/<context>/usecase`.
- Generated sqlc stays in `/db/sqlc` until the sqlc package is moved.
- Repository = CRUD + DB record ↔ domain map. Business rule stays usecase/domain.
- Create/update returns persisted entity when caller needs it. Error-only only when result irrelevant.
- Check nullable DB value validity before read. DB NULL → explicit domain absent state.
- Restore persisted VO with restore constructor. Never run input transform on stored data.
- Want sqlc table model return? Query projection and order must match model. Change SQL. Regenerate. Never hand-edit generated sqlc.

## Config and runtime

- Process env = runtime config. Env file works only if launch loads/exports it.
- App, migration, container DB config stay same.
- Health check names target service/resource explicit. No client default reliance.

## Test

- Split VO/entity tests by concept.
- Usecase service test orchestration, repository call, error flow. No repeated VO/entity matrix.
- Bug in DB map, nullable field, public error map → regression test.
- Stateful entity test uses fixed time, explicit fixture.

## Done

- Req met. Layer rule kept.
- Changed Go formatted.
- `make test` passes when workspace buildable.
- Generated output refreshed after source change.
