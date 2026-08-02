---
applyTo: "backend/**/*.go"
---

# Backend Go instructions

These instructions apply to the Go backend under `backend/`. The service uses Go 1.25, chi, pgx, sqlc, and swaggo. Keep changes consistent with the existing bounded contexts: `auth` and `task` may depend on `shared`, but must not depend on each other; `shared` must not depend on either context.

## Architecture

- `cmd/api/main.go` is the HTTP entry point. `internal/application` wires services, repositories, and unit-of-work implementations.
- Handlers own HTTP decoding, authentication/context extraction, status codes, and client-safe error mapping.
- Application services in `internal/<context>/usecase` orchestrate domain behavior, repository calls, and transaction boundaries. Repository interfaces belong in the usecase package.
- Domain code (`domain`) must not know about HTTP, SQL, repositories, transactions, drivers, or infrastructure context state. Prefer methods on entities, value objects, and aggregates for domain behavior.
- Repository implementations belong in `internal/<context>/repository` and handle CRUD, sqlc calls, and database-record/domain mapping. Keep business rules out of repositories.
- `db/queries` and `sqlc.yml` are sqlc sources; `db/sqlc` is generated code. Never hand-edit generated sqlc files. Regenerate them with `make sqlc-gen` after changing SQL.
- `docs/` is generated Swagger output. Regenerate it with `make swagger-gen` after changing API annotations.

## Design rules

- Use value-object constructors for input validation and separate restore constructors for persisted values. Do not duplicate validation in services.
- Use explicit absent/invalid domain state for nullable database values; check nullable validity before reading.
- Use one unit of work per bounded context. The service chooses the transaction boundary with `uow.Do(...)`; inside it, use only repositories supplied by the unit of work.
- Map known domain and database-constraint errors to stable, client-safe status/detail values. Never expose raw database errors, SQL, credentials, connection strings, stack traces, or other internal details in HTTP responses.
- For unknown failures, return a safe generic client error and retain the root error only for server diagnostics.
- Keep Swagger names and documented statuses aligned with actual handler behavior.

# Validation

From `backend/`, run the checks relevant to the change:

```bash
gofmt -l .             # must print nothing
go test ./...
go build ./cmd/api
```

`make test` runs the same tests with verbose output; `make go-fmt` formats files in place. For local development, `make dev` runs the API with Air in Docker. Follow the repository CI workflow in `.github/workflows/backend_ci.yml`, which checks formatting, tests, and builds the API with Go 1.25.

Before considering a change complete, verify behavior and error paths, consistency with the layer rules, absence of dead code and typos, generated-output updates where applicable, and that the relevant validation commands pass. If a command cannot be run, state the exact reason and the remaining verification gap.

# Review

When reporting review findings, use these levels consistently:

- `(Critical)`: a bug or verification failure that must be checked before accepting the change.
- No prefix: a standard concern that should be checked.
- `(Confirmation)`: ask why a design or implementation choice was made.
- `(NITS)`: a minor typo or formatting issue.
- `(Suggestion)`: a recommended improvement supported by evidence.
- `(Optional)`: a non-mandatory improvement.
- `(FYI)`: reference information that does not require action.

For `(Suggestion)` and `(FYI)`, cite evidence from the existing code, official documentation, or another relevant source. Keep review comments focused on behavior, consistency, dead code, typos, verification quality, rationale, and trade-offs.

Use this concise format for each review comment:

````text
(Level) Finding or question

Background:
Why this matters, including evidence or the expected behavior.

Example (if needed):
```go
// focused example or suggested change
```
````

Omit `Background` only when the finding is already self-explanatory. Do not add an example when it would not clarify the issue.
