# Context-owned read models and integration tests

## Approved outcome

- Project owns minimal progress inputs, repository reads, and pure domain aggregation. Remove the root Project progress adapters.
- Preserve one source of truth for Task percentages and recurring-occurrence eligibility. Extract only the pure policies actually reused by Task, Schedule, and Project; retain context-owned mutations.
- Remove redundant Tag ownership preflight adapters. Assignment SQL remains the atomic owner/permission check, with existing public errors preserved.
- Task reads its minimal Project information through its own repository using the existing SQL permission and lock contracts. Preserve same-transaction Project-first locking and rereads.
- Auth exposes a scalar timezone reader. Task and Schedule consume compatible primitive ports, eliminating root timezone adapters without changing timezone ownership or recurrence snapshots.
- Keep application database, Store, UOW, and UseCase assembly at the root. Domain contains no database I/O; plain read DTOs remain at DAO/port boundaries.
- Move real-database tests to `tests/integration/{project,task,schedule,tag,rest}` with minimal support in `tests/integration/internal/testdb`. Keep pure unit tests colocated.
- Separate unit and integration commands. Explicit integration execution requires a test database and must not silently skip tests. A single runner prepares an isolated disposable PostgreSQL database, applies migrations, runs the suite, and cleans up its own resources.

## Ownership

- `implement_project_progress`: Project production/domain/unit tests; Task/Schedule progress policy and unit tests; shared recurrence/progress policy; SQL sources and sqlc generation.
- `implement_adapter_cleanup`: root production wiring/adapters; Task Project projection; Tag preflight removal; Auth scalar timezone reader; Task/Schedule common usecase repository-port files and associated unit tests.
- `implement_integration_layout`: all real-DB integration test files and migration of their coverage; Make/test runner configuration; backend guidance and current documentation.
- Main: decisions, this plan, coordination, and review of delegated findings.

Shared-file changes require an explicit handoff. The progress owner alone edits SQL and generates sqlc. No agent resets a development database or edits another owner's files concurrently.

## Implementation gates

1. Publish Project input/policy/constructor contracts and adapter-removal interfaces.
2. Extract shared pure policy while preserving existing semantics; implement Project-owned read projection and domain aggregate.
3. Update root wiring and remove obsolete adapters/ports. Preserve authorization errors, actor/history behavior, row-lock order, and multi-context transaction boundaries.
4. Move DB tests using final public APIs; retain unique assertions and deterministic concurrency regressions. Use an integration build tag and central test DB setup/guard.
5. Run unit tests, isolated integration tests without individual skips, backend build, generated consistency, and diff checks. Review context dependencies and stale guidance.

## Required regressions

- Per-Task rounding before Project averaging; completed Task behavior; empty, Task-only, Schedule-only, and mixed Projects.
- Saved override/skip precedence, live overrides after root deletion, local-today virtual occurrences, future exclusion, timezone/DST handling, and existing invalid-input behavior.
- Same-series Schedule tags through root/saved occurrences and list projections; no Tag-only revision increments; owner and permission denials.
- Project deletion/member removal atomic rollback and actor/revision attribution; child-create/delete, TodoItem insertion/deletion, and reassignment races.
- Task Project association permission/error behavior and Project-first locking within the Task UOW.
- Auth timezone lookup and recurrence metadata/date initialization.

No API, schema, or frontend feature changes are planned. Any necessary public behavior decision must be reported before changing it.

## Validation

- Removed the four root read adapters; context imports remain independent. Project reads Task facts from the existing shared SQL projection and Schedule facts in two batched queries, then uses pure domain/shared policy.
- Targeted unit suites and backend build passed. Independent policy and integration-runner reviews found no actionable regressions.
- `make test` passed with integration tests excluded by build tags.
- `make test-integration` passed against freshly migrated disposable PostgreSQL: 54 tests (Project 14, REST 1, Schedule 14, Tag 1, Task 24), including DST, rollback, and concurrency regressions.
- Direct tagged execution without `INTEGRATION_DATABASE_URL` failed as required. No integration tests were skipped.
- The runner cleaned its own container; existing development and audit databases were untouched. Shell syntax and `git diff --check` passed.
