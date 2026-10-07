# Logging guide

These rules govern logging across the backend: `cmd/api`, REST, application
usecases, telemetry, and `internal/logging/`. Read this guide when changing
logging behavior in any of those layers. See the
[application guide](../application/AGENTS.md) for application
usecase/domain/repository logging details.

## Shared contract and format

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
  and `version`. Resolve service metadata once for logs and trace resources.
  See the [telemetry guide](../telemetry/AGENTS.md) for metadata defaults and
  override precedence. `LOG_LEVEL` defaults to `INFO`.

## HTTP / API layer

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

## Runtime logging

- `cmd/api` creates and injects the logger and adapters, and records startup,
  shutdown, and initialization failures. Include a safe `operation` field for
  failures. Route OTel asynchronous errors and `net/http` server errors through
  structured logging as well.

## Safe fields and levels

- DEBUG: selected diagnostic details. INFO: request lifecycle and significant
  successful operations. WARN: expected rejection. ERROR: unexpected failure,
  HTTP 5xx, or panic/abort. Keep messages stable and put identifiers and
  operation names in structured fields.
- Never log passwords or hashes, access/page tokens, Authorization/Cookie
  headers, connection strings, request bodies, free-form user content, raw SQL
  or parameters, raw error strings, or panic payloads/stacks. Use safe fields
  such as operation, resource IDs, error type, and SQLSTATE when available.

PostgreSQL owns database diagnostics, statement/slow-query logging, and native
log output. Infrastructure owns collection, retention, and forwarding of
those server logs, for example with an external OTel Collector. Manage
PostgreSQL logging settings independently of application logging.
