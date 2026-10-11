# Logging and telemetry guide

These rules cover structured logging, OpenTelemetry trace setup, propagation,
and lifecycle across the backend: `cmd/api`, REST, application usecases, and
`internal/telemetry/`. Read this guide when changing logging or trace behavior in
any of those layers. See the [backend guide](../../AGENTS.md) for commands and
the [application guide](../application/AGENTS.md) for application boundaries.

## Shared logger contract and format

- `internal/telemetry` owns the `telemetry.Logger` contract and its `log/slog`
  implementation. Write structured JSON to stdout through this package.
- Keep the interface to `Debug`, `Info`, `Warn`, and `Error`, each accepting
  `context.Context`, a stable message, and structured key/value fields. Inject
  the logger through constructors; keep logger configuration in `cmd/api`.
- Application wiring, usecases, and REST code may depend on
  `internal/telemetry`. Usecases must not depend on concrete slog or OTel SDK
  types. Domain models and DAOs must not depend on a logger.
- Pass the operation's existing context to every log call. The logger attaches
  `request_id` and valid span context fields (`trace_id`, `span_id`,
  `trace_flags`) automatically; callers must not copy these fields manually.
- Common fields are `timestamp`, `level`, `message`, `service`, `environment`,
  and `version`. `config.Load` resolves service metadata once for logs and trace
  resources, then passes `ApplicationConfig` and `LoggerConfig` to `Setup` and
  `NewLogger`. Logger config holds resource attributes and log level;
  `LOG_LEVEL` defaults to `INFO`.

## HTTP / API logging

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
- Use DEBUG for selected diagnostic details, INFO for request lifecycle and
  significant successful operations, WARN for expected rejection, and ERROR
  for unexpected failure, HTTP 5xx, or panic/abort. Keep messages stable and
  put identifiers and operation names in structured fields.
- Never log passwords or hashes, access/page tokens, Authorization/Cookie
  headers, connection strings, request bodies, free-form user content, raw SQL
  or parameters, raw error strings, or panic payloads/stacks. Use safe fields
  such as operation, resource IDs, error type, and SQLSTATE when available.
- PostgreSQL owns database diagnostics, statement/slow-query logging, and
  native log output. Infrastructure owns collection, retention, and forwarding
  of those server logs, for example with an external OTel Collector. Manage
  PostgreSQL logging settings independently of application logging.

## SDK, resource, and propagation

- `internal/telemetry` owns official OpenTelemetry SDK setup, shared resource
  metadata, propagation, sampling, and trace exporter configuration.
- `config.Load` resolves metadata defaults `OTEL_SERVICE_NAME=api`,
  `ENVIRONMENT=development`, and `APP_VERSION=1.0.0` for logs and trace
  resources. OpenTelemetry resource attributes may override environment and
  version; `OTEL_SERVICE_NAME` takes precedence over `service.name` in
  `OTEL_RESOURCE_ATTRIBUTES`. `Setup` uses the passed application metadata and
  `LoggerConfig.ResourceAttributes` when building SDK resource metadata.
- Use W3C Trace Context and Baggage propagation. Instrument HTTP server spans
  with official `otelhttp`; obtain span context from OpenTelemetry. Do not parse
  `traceparent` or generate trace IDs manually.
- Sampling defaults to `parentbased_always_on`. `OTEL_TRACES_SAMPLER` and
  `OTEL_TRACES_SAMPLER_ARG` select another supported sampler or ratio.

## Export and shutdown

- Configure OTLP export with standard `OTEL_*` environment variables. Support
  HTTP/protobuf and gRPC, plus standard endpoint, headers, timeout, compression,
  and TLS settings. If `OTEL_TRACES_EXPORTER` is unset, it defaults to `otlp`;
  the default protocol is `http/protobuf`. Set
  `OTEL_EXPORTER_OTLP_PROTOCOL=grpc` to select gRPC.
- Local `.env.example` sets `OTEL_TRACES_EXPORTER=none`. `none` disables trace
  export while retaining SDK span context. The repository does not run an OTLP
  Collector; configure an external Collector when exporting traces.
- After HTTP shutdown, flush and shut down the trace provider with bounded
  contexts.
