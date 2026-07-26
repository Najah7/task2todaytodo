# Backend guide

## Map

```text
cmd/api/                 API entry
db/migrations/           PostgreSQL schema
db/queries/              sqlc SQL source
docs/                    Generated Swagger
internal/adapters/       External adapters
internal/application/    Wiring, config
internal/auth/domain/    Auth VO, entity, domain tests
internal/auth/usecase/   Auth application services, repository ports, usecase tests
internal/auth/repository/ Auth DB adapters, sqlc mapping
internal/task/domain/    Task VO, entity, aggregate, domain tests
internal/task/usecase/   Task application services, repository ports, usecase tests
internal/task/repository/ Task DB adapters, sqlc mapping
internal/domain/shared/  Shared VO/helpers used by multiple contexts
internal/handlers/       HTTP boundary
internal/middlewares/    HTTP middleware
/db/sqlc/ Generated sqlc package
```

`db/queries` + `sqlc.yml` = sqlc source. `/db/sqlc` = generated.

## Commands

- `make go-fmt` — format.
- `make test` — test before handoff.
- `make build` — validate API binary.
- `make swagger-gen` — after Swagger/public response change. Commit `docs` output.

Live reload build fail = app stale. Check build log, binary, port-owning process.

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

### VO

- One validated concept.
- Validate in constructor.
- Keep representation work, e.g. hash, here.
- Separate input constructor from persisted-state restore when invariants differ.

### Entity

- Factory create. No external struct literal.
- Own invariant, state change, state query.
- New-state and restored-state factory may differ.
- Model absent state explicit. Never infer state from invalid DB data.
- `NewXXX`: minimal required fields only. Keep create path simple.
- `NewXXXWithDetails`: richer create path when optional/detail fields are provided.
- `NewExistingXXX`: restore persisted state. Accept all stored fields, including timestamps.
- `NewZeroXXX`: explicit absent/invalid return value.

### Aggregate

- Use Aggregate for read models that span multiple tables or entity collections.
- JOIN-based queries should return repository-mapped Aggregates, e.g. `ProjectAggregate` for Project + Tasks or `TaskAggregate` for Task + TodoItems + TaskSchedules.
- Keep SQL/JOIN details inside repository implementations. Domain Aggregates describe the composed domain result, not database mechanics.
- Do not force a single-table Entity to carry child collections only because one API response needs them.

### Service

- Service means Application Service unless explicitly stated otherwise.
- Orchestrate domain object load, domain method call, repository save, and transaction boundary.
- Keep repository interfaces in the usecase package, close to the Application Service that needs them.
- No duplicate VO/entity validation.
- No HTTP req/context. Handler extracts transport data, passes args.
- Pre-check helps. DB constraint final authority.
- Prefer entity/VO/aggregate methods for domain behavior.
- Avoid Domain Service by default. Add domain behavior to the domain object when it naturally belongs there.
- If behavior cannot be expressed cleanly as a domain method, implement the orchestration in Application Service.
- Introduce Domain Service only for rare pure-domain behavior that does not belong to any entity/VO and does not need repository/DB access.

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
