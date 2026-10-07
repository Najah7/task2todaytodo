# Backend

The backend is a Go HTTP API. Business rules live in `internal/application`; the
REST boundary lives in `internal/port/rest`. PostgreSQL migrations and query
sources live under `db/`. Runtime configuration, structured logging, and trace
setup belong to `internal/config`, `internal/logging`, and `internal/telemetry`.

Start the backend with the [repository quick start](../README.md#backend-quick-start).
Run `make help` in this directory for available commands. Common developer
commands:

```bash
make test                 # unit tests
make test-integration     # isolated PostgreSQL integration tests (Docker required)
make build                # build API
make sqlc-gen             # regenerate sqlc from SQL
make swagger-gen          # regenerate OpenAPI docs
```

The API reference is available at <http://localhost:8080/swagger/index.html>
when the server is running. See the [database guide](db/AGENTS.md),
[application guide](internal/application/AGENTS.md), and [REST guide](internal/port/rest/AGENTS.md)
for implementation boundaries.

## Logs and traces

The API writes structured JSON logs to stdout. Request logs carry a request ID
and, when available, trace context. Local [`.env.example`](.env.example)
disables trace export; configure an external Collector to export traces. The
repository does not start a Collector. See the [logging guide](internal/logging/AGENTS.md)
and [telemetry guide](internal/telemetry/AGENTS.md) for implementation rules.
