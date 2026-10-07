# REST guide

These rules apply to handlers and middleware under `internal/port/rest/`.
See the [application guide](../../application/AGENTS.md) for usecases and read
models; see the
[logging guide](../../logging/AGENTS.md) for request and runtime logging
policies.

## Handler and public error

- `internal/port/rest` owns request decoding, auth/context extraction, HTTP
  status, and error mapping. `internal/port/rest/middleware` owns HTTP
  middleware.
- Every error response needs stable, client-safe detail.
- Map known domain and database constraint errors to stable status, field, and
  detail codes.
- Never return raw database errors, connection strings, SQL, passwords, or
  stack traces. Unknown failures get a safe generic detail; keep the root error
  for server diagnostics.
- Swagger type names and statuses must match handler behavior.

## List APIs

- `internal/port/rest/pagination` owns HTTP query parsing and encrypted page
  tokens. `internal/port/rest/fieldmask` owns response field selection.
  `internal/port/rest/list_pagination.go` applies these shared HTTP behaviors.
- `internal/config` loads process settings, including validated
  `PAGE_TOKEN_KEY`. `cmd/api/main.go` creates the REST token codec and passes it
  to handlers.
- Do not put REST token or response types in application usecases. List
  usecases own page requests and cursor boundaries; repositories own
  resource-specific keyset ordering, ownership conditions, and SQL. See the
  [application guide](../../application/AGENTS.md) for those rules.
