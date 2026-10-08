# Application guide

These rules apply to `internal/application/`, including the composition root
and each business context. See the [backend guide](../../AGENTS.md) for commands
and real-database integration tests. See the [REST guide](../port/rest/AGENTS.md),
[configuration guide](../config/AGENTS.md), [SQL guide](../../db/AGENTS.md), and
[logging guide](../logging/AGENTS.md) for adjacent boundaries.

## Structure and boundaries

```text
internal/application/<context>/{domain,dao,usecase,repository}
internal/application/shared/              Cross-context contracts and pure policy
internal/application/                      Composition root and transaction wiring
```

`<context>` is `auth`, `project`, `task`, `schedule`, or `tag`. Keep each
context's domain, DAO, usecase, and repository within that context. Context
code may depend on its own layers and on `application/shared`, as shown in the
[backend layer map](../../AGENTS.md#layer-rule).

Business contexts must not import one another's packages. The application root
may compose multiple contexts, `port/rest` may consume multiple contexts at the
transport boundary, and integration scenarios may exercise multiple contexts.
These are composition, transport, and test boundaries; they do not permit
business-context dependencies.

`application/shared` may contain contracts or pure policy shared by contexts,
but must not depend on a business context or `port/rest`. Keep Task and Schedule
independent. The shared Tag catalog is its own context; Task and Schedule each
own their Tag-assignment operations and use primitive owner checks. Define
cross-context reads and writes at the consuming usecase boundary, then inject
their implementations through root wiring. Keep HTTP query parsing, field
masks, cursor tokens, and response types in `port/rest`.

## Domain

Domain code must not import HTTP, database, framework, or driver packages, or
depend on repositories, transactions, or context-carried database state. It
owns business state and behavior.

### VO

- Name the file after the domain concept. Do not add suffixes such as `_value`,
  `_entity`, or `_service`.
- Keep each VO to one validated concept and validate it in the constructor.
- Keep representation work, such as hashing, with the value.
- Separate input construction from persisted-state restoration when their
  invariants differ.

### Entity

- Name the file after the domain concept; do not add an `_entity` suffix.
- Create entities through factories; do not allow callers to create them with
  struct literals.
- The entity owns its invariants, state changes, and state queries. New-state
  and restored-state factories may differ.
- Model absent state explicitly. Never infer absence from invalid database
  data.
- `NewXXX` accepts only minimal required fields. Use `NewXXXWithDetails` for a
  richer create path with optional or detailed fields.
- `NewExistingXXX` restores all persisted fields, including timestamps.
- `NewZeroXXX` represents an explicit absent or invalid return value.

## DAO (read model)

- Define read DAOs in the context's `dao/` package. Use primitives or
  read-only types composed mainly of primitives. You may nest simple read
  types when that makes returned data clearer.
- Do not put domain Value Objects or Entities in a DAO. DAOs do not define
  domain behavior, validation, or invariants.
- Repositories map database records to DAOs. Return the persisted DAO from
  create/update when the caller needs it.
- A cross-context read model belongs to the consuming context. Its repository
  may use SQL/ACL queries to project only the primitive facts that consumer
  needs. Do not expose the source context's DAO or domain types. Read DTOs may
  differ by consumer. Keep domain behavior and update policy with the owner;
  do not duplicate them in a read model. Projections still depend on the shared
  database schema, so a schema change may require updates to multiple context
  repositories.
- Pass a DAO through when no domain behavior is needed. Otherwise, the usecase
  converts its fields to value objects and domain models through existing
  factories. Do not make domain factories accept DAOs.

```text
WRITE: Domain Model -> Repository -> DAO (persisted result, when needed)
READ:  Repository -> DAO -> UseCase/REST handler
                    -> Domain Model only when domain behavior is needed
```

## Use case

- Give each application operation its own `*UseCase` type. Do not group
  operations as methods on a resource `*Service`.
- Name the file after the operation, expose it through `Execute(ctx, ...)`,
  and name its constructor `New<Operation>UseCase`.
- Treat reads as usecases; do not create a separate application `query`
  package. Orchestrate domain loading, domain methods, persistence, and the
  transaction boundary required by each operation.
- `application/shared/pagination` owns page-size policy and the
  `LIMIT page_size + 1` calculation. Each List usecase owns its page request
  and cursor boundary. Its repository owns resource-specific keyset ordering,
  ownership conditions, and SQL.
- Keep repository and UOW interfaces in the usecase package near the usecases
  that need them. Group related usecases for wiring only; keep behavior in each
  operation.
- Do not duplicate VO/entity validation or define HTTP request/response types.
  The handler extracts transport data and passes arguments. `context.Context`
  carries cancellation and deadlines.
- A pre-check helps, but the database constraint is the final authority.
- Prefer entity and VO methods for domain behavior. Orchestrate behavior in the
  usecase when it does not fit a domain method. Avoid Domain Services by
  default; add one only for rare pure-domain behavior that belongs to no
  entity or VO and needs no repository or database access.

## Transaction

- Use the Unit of Work pattern. The usecase owns its UOW and repository
  interfaces. The application root implements database begin, commit, and
  rollback. The UOW accepts a workflow callback such as
  `uow.Do(ctx, func(ctx, repos) error { ... })`.
- Do not queue operations through event registration. Prepare one UOW per
  bounded-context concern.
- A UOW normally exposes repositories for its context. A cross-context
  orchestration UOW may additionally expose only narrow, consumer-defined
  command ports with primitive inputs. Root wiring implements them through
  owner repositories bound to the same transaction. Project member removal
  coordinates Task and Schedule reassignment commands this way. Project
  deletion changes only the parent Project row and does not invoke child bulk
  commands. Do not expose foreign repositories or domain types through the UOW.
- Repository implementations may expose `WithTx(tx)` internally; do not add it
  to usecase repository ports. The usecase decides the transaction boundary.
- The UOW guarantees atomicity and rollback. It does not own business policy,
  enforce ownership, or replace validation in the owning context. Inside the
  callback, perform every database write, and every database read that enforces
  transaction invariants, through the transaction-bound repositories or ports
  in `repos`. Do not use non-transaction repositories for those operations.
- Task and Schedule currently read the user's timezone inside the UOW callback
  through a non-transaction-bound scalar reader. This returns a committed-value
  snapshot; it does not guarantee transaction invariants. Do not generalize
  this exception to non-transactional writes or other invariant reads.

## Repository

- Implementations live in `internal/application/<context>/repository`; ports
  live in that context's `usecase` package.
- Repositories handle CRUD, database-record-to-DAO mapping, and
  domain-model-to-database mapping. Business rules stay in usecases and domain.
  Return persisted DAOs when callers need them; otherwise return only an error.
- For writes owned by the context, use domain models and their methods to
  validate changes and protect invariants. For cross-context orchestration, a
  consuming usecase may define a narrow command port with primitive inputs and
  delegate an explicit bulk operation (such as child deletion or reassignment)
  to the owning repository through root wiring. Root wiring must bind that
  repository to the same transaction. Keep owner rules in the owning context;
  do not copy or bypass them in the consumer. Per-row domain reconstruction is
  unnecessary when the owner-scoped bulk command preserves required invariants
  and no additional per-entity behavior is needed. Database foreign-key
  cascades may handle referential cleanup such as Tag-assignment join rows, but
  the owning command must still perform required business checks.
- Check nullable database values before reading them. Map `NULL` to an explicit
  absent domain state. When restoring domain state from a DAO, use the correct
  VO restore constructor; do not apply input transforms to stored data.
- Keep query source, generated-code location, and regeneration instructions in
  the [SQL guide](../../db/AGENTS.md#sql-generation). Never hand-edit generated
  sqlc code.

## Tests

- Keep pure domain, usecase, and database-config tests with their packages;
  they must not require PostgreSQL. The [backend guide](../../AGENTS.md#test)
  documents real-database integration tests and how to run them.
- Split VO/entity tests by concept. Name each usecase test after its
  implementation file; for example, `create_user_test.go` for `create_user.go`.
- Usecase tests cover orchestration, repository calls, and error flow; do not
  repeat the VO/entity test matrix.
- Use fixed time and explicit fixtures for stateful entity tests.

## Logging

- Usecases may depend on the `internal/logging` contract, but not concrete slog
  or OTel implementations. Domain models and DAOs must not depend on a logger.
- Log authentication outcomes, security events, and important state operations,
  such as task status changes and occurrence complete/reopen/skip/restore/
  reschedule. Use INFO for success. Use WARN for authentication rejection and
  include a stable, safe reason.
- Emit success logs only after persistence succeeds. For a transaction, emit
  them only after `UOW.Do` returns nil, which means commit succeeded. Never log
  success inside the callback or after rollback or commit failure. Describe
  idempotent commands as completed operations unless an actual change is known.
- Log unexpected repository, transaction, and persisted-state restoration
  failures at ERROR. Include an `operation` field and `logging.ErrorFields(err)`.
  Invalid stored state is unexpected, even if its error is also used for input
  validation.
- Ordinary CRUD success and expected validation, not-found, conflict,
  cancellation, or deadline errors need no additional usecase log. Middleware
  records the HTTP response outcome.
- Log each failure once, at the boundary that owns it. A delegating usecase
  must not repeat the callee's failure log; it logs failures from its own
  preliminary repository or UOW work.
- Repositories and UOWs return errors to usecases. Do not log application SQL,
  driver details, per-query duration, or transaction lifecycle there. The
  usecase owns the application failure event.
