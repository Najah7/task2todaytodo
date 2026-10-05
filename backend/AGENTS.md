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
internal/telemetry/                       OpenTelemetry resource, propagation and trace exporter
internal/application/task/time.go        Task-context calendar and wall-time helpers
internal/port/rest/                      HTTP handlers, response and error mapping
internal/port/rest/middleware/           HTTP middleware
internal/port/rest/pagination/           Query parsing and encrypted page tokens
internal/port/rest/fieldmask/            Read response field masks
internal/port/adapter/                   External adapters, including ULID and UUID
```

`<context>` is `auth` or `task`. HTTP handlers for both contexts live in the
single `internal/port/rest` package.

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
cmd/api -> port/rest + application wiring
port/rest and middleware -> application/<context>/usecase, dao/domain
application/<context>/usecase -> own dao/domain + application/shared
application/<context>/repository -> own usecase ports + dao/domain + sqlc/DB
application root -> usecase + repository impl + UOW + port/adapter wiring
```

Domain has no HTTP, DB, framework, or driver imports. Usecase owns repository
ports and must not import `internal/port/rest`. Repository impl imports usecase
ports, maps DB records to DAOs, and maps domain models to DB write parameters.
The root `internal/application` package wires concrete repositories, UOWs, and
usecases. `cmd/api` wires HTTP handlers and creates the REST page-token codec.

`internal/application/<context>/` owns one bounded context. Keep its domain,
DAO, usecase, and repository inside that context. Use `internal/application/shared`
only for transport-independent contracts or policy shared by contexts. HTTP
query parsing, field masks, and cursor-token encoding belong to `internal/port/rest`.

### Context dependencies

```text
application/auth, application/task -> application/shared
port/rest -> application/auth, application/task, application/shared
cmd/api -> application root, port/rest, port/adapter
```

- Do not import one context from another: `task -> auth`, `auth -> task`, and
  `shared -> auth/task` are forbidden.
- A context may import `application/shared`, but shared must not depend on a
  context or on `port/rest`.
- `port/rest` may depend on usecases and read models. Application usecases must
  not depend on REST query, token, field-mask, or response types.
- If a cross-domain reference is unavoidable, define the required interface on
  the importing side and inject the implementation from `main.go` or
  `application.go`.
- These rules exist to prevent circular imports.

### VO

- Use the domain concept name as the file name. Do not add type suffixes such as
  `_value`, `_entity`, or `_service`.
- One validated concept.
- Validate in constructor.
- Keep representation work, e.g. hash, here.
- Separate input constructor from persisted-state restore when invariants differ.

### Entity

- Use the domain concept name as the file name. Do not add an `_entity` suffix.
- Factory create. No external struct literal.
- Own invariant, state change, state query.
- New-state and restored-state factory may differ.
- Model absent state explicit. Never infer state from invalid DB data.
- `NewXXX`: minimal required fields only. Keep create path simple.
- `NewXXXWithDetails`: richer create path when optional/detail fields are provided.
- `NewExistingXXX`: restore persisted state. Accept all stored fields, including timestamps.
- `NewZeroXXX`: explicit absent/invalid return value.

### DAO (Read Model)

- DAO is the read model. Define it in `internal/application/<context>/dao/` and
  use it for repository results intended for reading.
- Model DAO fields with primitives or read-only types composed mainly of
  primitives. Nesting these simple read types is allowed when it makes the
  returned data clearer, such as `Project.Type` using a DAO `ProjectType`.
- Do not put domain Value Objects or Entities in a DAO. DAO types do not define
  domain behavior, validation, or invariants; keep that modeling in `domain/`.
- Repository maps database records to DAO for reads. Return DAO from create or
  update when the caller needs the persisted result.
- For a read that needs no domain behavior, pass the DAO to the caller as-is.
  When an operation needs domain behavior, the use case converts DAO fields to
  value objects and a domain model using the existing domain factories. Do not
  make domain factories accept DAO values.
- Use a domain model for writes. Value Objects and domain methods validate
  changes and protect invariants; pass the resulting domain model to the
  repository.

```text
WRITE: Domain Model -> Repository -> DAO (persisted result, when needed)
READ:  Repository -> DAO -> UseCase/REST handler
                    -> Domain Model only when domain behavior is needed
```

### Use Case

- Model one application operation per `*UseCase` type, such as
  `CreateUserUseCase` or `AuthenticateUseCase`. Do not group several operations
  as methods on a resource `*Service`.
- Name the file after the operation, such as `create_user.go` or
  `authenticate.go`. Expose the operation through `Execute(ctx, ...)` and name
  its constructor `New<Operation>UseCase`.
- Treat reads as use cases too. Do not create a separate application `query` package for them.
- Orchestrate domain object load, domain method call, repository save, and transaction boundary as the operation requires.
- Keep repository interfaces in the usecase package, close to the use cases that need them.
- Group related use cases by resource for wiring only, such as `UserUseCases`
  and `AccessTokenUseCases`. `application.UseCase` holds those groups; `main.go`
  passes each resource group to its handler. Keep business behavior in the
  individual use cases.
- No duplicate VO/entity validation.
- No HTTP request or response types. Handler extracts transport data and passes
  arguments; `context.Context` carries cancellation and deadlines.
- Pre-check helps. DB constraint final authority.
- Prefer entity and VO methods for domain behavior.
- Avoid Domain Service by default. Add domain behavior to the domain object when it naturally belongs there.
- If behavior cannot be expressed cleanly as a domain method, implement the orchestration in the use case.
- Introduce Domain Service only for rare pure-domain behavior that does not belong to any entity/VO and does not need repository/DB access.

### Transaction

- Implement transaction support with the Unit of Work pattern.
- Usecase owns `UOW` and `Repositories` interfaces.
- The root `internal/application` package implements `UOW` with DB begin,
  commit, and rollback.
- In Go, `UOW` receives the workflow function, e.g. `uow.Do(ctx, func(ctx, repos) error { ... })`.
- Do not use an event-registration style where operations are queued and executed at the end.
- Prepare one Unit of Work per domain concern/bounded context, e.g. auth UoW and task UoW.
- A UoW exposes only repositories for its own bounded context.
- Repository impl may expose `WithTx(tx)` internally. Do not add `WithTx` to usecase repository ports.
- The use case decides the transaction boundary by calling `uow.Do(...)`.
- Inside `uow.Do(...)`, use only repositories received from `repos`. Do not call use case fields backed by non-transaction repositories.
- Domain never knows transaction, repository, driver, or context-carried DB state.

## REST handler and public error

- `internal/port/rest` owns decode, auth/context extract, HTTP status, and error
  mapping. `internal/port/rest/middleware` owns HTTP middleware.
- Every error res needs stable, client-safe detail.
- Map known domain and DB constraint errors → stable status, field, detail code.
- Never return raw DB error, connection string, SQL, password, stack trace.
- Unknown failure → safe generic detail. Keep root error for server diagnostics.
- Swagger type names and statuses match real handler behavior.

### List APIs

- `internal/application/shared/pagination` owns the page-size policy and
  `LIMIT page_size + 1` window calculation. List usecases may import it.
- `internal/port/rest/pagination` owns HTTP query parsing and encrypted page
  tokens. `internal/port/rest/fieldmask` owns response field selection.
  `internal/port/rest/list_pagination.go` applies these shared HTTP behaviors.
- Each List usecase owns its page request and cursor boundary. Its repository
  owns the resource-specific keyset order, owner condition, and SQL query.
- `internal/config` loads process settings, including validated `PAGE_TOKEN_KEY`.
  `cmd/api/main.go` creates the REST token codec and passes it to handlers. Do not
  put REST token or response types in application usecases.

## Repository and SQL

- Repository impl lives in `internal/application/<context>/repository`.
- Repository ports live in `internal/application/<context>/usecase`.
- Generated sqlc stays in `/db/sqlc` until the sqlc package is moved.
- Repository = CRUD + DB record ↔ DAO mapping + Domain Model → DB mapping.
  Business rule stays usecase/domain.
- Create/update returns persisted DAO when the caller needs it. Error-only only
  when the result is irrelevant.
- Check nullable DB value validity before read. DB NULL → explicit domain absent state.
- When restoring domain state from DAO, use the appropriate VO restore
  constructors in the use case. Never run input transforms on stored data.
- Want sqlc table model return? Query projection and order must match model. Change SQL. Regenerate. Never hand-edit generated sqlc.

## Config and runtime

- Process env = runtime config. Env file works only if launch loads/exports it.
- `internal/config` owns process environment loading and validation. Pass database
  settings into `internal/application`; application code must not read process
  environment. `cmd/api/main.go` creates the REST cursor-token codec from the
  validated `PAGE_TOKEN_KEY` and passes it to handlers.
- Keep DB settings consistent across the application, migration command, and
  container configuration.
- Health check names target service/resource explicit. No client default reliance.

## Logging

### Shared contract and format

- `internal/logging` owns the `logging.Logger` contract and its `log/slog`
  implementation. Write structured JSON to stdout through this package.
- Keep the interface to `Debug`, `Info`, `Warn`, and `Error`, each accepting
  `context.Context`, a stable message, and structured key/value fields. Inject
  the logger through constructors; keep logger configuration in `cmd/api`.
- Application wiring, usecases, and REST code may depend on `internal/logging`.
  Usecases must not depend on the concrete slog implementation or OTel SDK.
- Pass the operation's existing context to every log call. The logger attaches
  `request_id` and valid span context fields (`trace_id`, `span_id`, `trace_flags`)
  automatically; callers must not copy these fields manually.
- Common fields are `timestamp`, `level`, `message`, `service`, `environment`,
  and `version`. Resolve service metadata once for both logs and trace resources.
  Defaults: `OTEL_SERVICE_NAME=api`, `ENVIRONMENT=development`,
  `APP_VERSION=1.0.0`, `LOG_LEVEL=INFO`. OTel resource attributes may override
  metadata; `OTEL_SERVICE_NAME` takes precedence for the service name.

### HTTP / API layer

- Request logging middleware owns the mechanical `request started` and
  `request completed` events for all API requests. Keep this boilerplate out of
  handlers; handlers own transport parsing, error mapping, and responses.
- Start events use INFO and include the HTTP method. Completion events include
  `http.method`, `http.path`, `http.status_code`, and numeric `duration_ms`.
  Use the matched route template for the path, or `unmatched`; never log raw
  paths or query strings. Completion level: below 400 = INFO, 4xx = WARN,
  5xx or panic/abort = ERROR.
- Inherit a valid canonical UUID from `X-Request-ID`, normalized to lowercase.
  Generate a UUID otherwise through the UUID adapter injected as `shared.ID`.
  Consumers depend on the ID interface; concrete adapter wiring belongs in
  `cmd/api`. Return the same ID in the response header and public error body.
- Middleware owns panic reporting with safe type metadata. Return a generic
  500 response before headers are committed; abort an already committed
  response. Never log panic payloads or stacks.

### Usecase layer

- Add logs intentionally for authentication outcomes, security events, and
  important state operations such as task status changes and occurrence
  complete/reopen/skip/restore/reschedule. Use INFO for successful operations
  and WARN for authentication rejection with a stable, safe reason.
- Emit success only after persistence succeeds. For transactions, log after
  `UOW.Do` returns nil, including commit. Never emit success from inside the
  transaction callback or after rollback/commit failure. Describe idempotent
  commands as completed operations unless an actual change is known.
- Log unexpected repository, transaction, and persisted-state restoration
  failures at ERROR with an `operation` field and `logging.ErrorFields(err)`.
  Invalid stored state is an unexpected failure even when its error is also
  used for input validation.
- Ordinary CRUD success and expected validation, not-found, conflict,
  cancellation, or deadline errors do not need additional usecase logs.
  HTTP middleware still records their response outcome.
- Log a failure once at the boundary that owns it. A delegating usecase must
  not repeat the callee's failure log; it logs failures from its own preliminary
  repository or UOW work.

### Domain and DAO layers

- Domain models and DAOs have no logger dependency. Domain code returns errors
  and models state; the usecase decides which outcomes deserve a log.

### Repository, UOW, and database layers

- Repositories and UOW return errors to the usecase. Keep application SQL,
  driver, per-query duration, and transaction lifecycle logging out of these
  implementations; the usecase owns the application failure event.
- PostgreSQL owns database diagnostics, statement/slow-query logging, and
  native log output. Infrastructure owns collection, retention, and forwarding
  of those server logs, for example with an external OTel Collector.
- Manage PostgreSQL logging settings and log collection in infrastructure
  configuration independently of application logging. Preserve the underlying
  error for classification, while logging only safe metadata in the API.

### Runtime and telemetry layers

- `cmd/api` creates and injects the logger and adapters, and records startup,
  shutdown, and initialization failures. Include a safe `operation` field for
  failures. Route OTel asynchronous errors and `net/http` server errors through
  structured logging as well.
- `internal/telemetry` owns official OTel SDK setup, shared resource metadata,
  W3C Trace Context/Baggage propagation, sampling, and OTLP export configuration.
  Use official `otelhttp` instrumentation for HTTP server spans; obtain span
  context from OTel instead of parsing `traceparent` or generating trace IDs.
- OTLP export uses standard `OTEL_*` variables and supports HTTP/protobuf and
  gRPC. `OTEL_TRACES_EXPORTER=none` disables export while retaining SDK span
  context. Flush and shut down the provider with bounded contexts after HTTP
  shutdown. Application JSON logs remain on stdout.

### Safe fields and levels

- DEBUG: selected diagnostic details. INFO: request lifecycle and significant
  successful operations. WARN: expected rejection. ERROR: unexpected failure,
  HTTP 5xx, or panic/abort. Keep messages stable and put identifiers and
  operation names in structured fields.
- Never log passwords or hashes, access/page tokens, Authorization/Cookie
  headers, connection strings, request bodies, free-form user content, raw SQL
  or parameters, raw error strings, or panic payloads/stacks. Use safe fields
  such as operation, resource IDs, error type, and SQLSTATE when available.

## Test

- Split VO/entity tests by concept.
- Name each use case test file after its implementation file with `_test.go`,
  such as `create_user_test.go` for `create_user.go`.
- Use case tests cover orchestration, repository calls, and error flow. Do not
  repeat the VO/entity test matrix.
- Bug in DB map, nullable field, public error map → regression test.
- Stateful entity test uses fixed time, explicit fixture.

## Done

- Req met. Layer rule kept.
- Changed Go formatted.
- `make test` passes when workspace buildable.
- Generated output refreshed after source change.
