# Runtime configuration guide

These rules cover process configuration in `internal/settings` and its use by
`cmd/api`, the application, migrations, and container configuration. Read this
guide when changing runtime configuration in any of those areas.

- The process environment is runtime configuration. An env file works only if
  the launch process loads or exports it.
- `internal/settings` owns process environment loading and validation. Pass
  database settings into `internal/application`; application code must not read
  process environment.
- `internal/settings` resolves shared service, environment, and version metadata
  once and parses `OTEL_RESOURCE_ATTRIBUTES` for application and telemetry
  setup. Identity fields live in `Config.Application`; resource attributes and
  log level live in `Settings.Logger`. Sampling and exporter settings remain
  owned by the OpenTelemetry SDK and its environment configuration.
- `cmd/api/main.go` passes the validated `PAGE_TOKEN_KEY` to `rest.NewHandler`,
  which creates the shared REST cursor-token codec.
- Keep database settings consistent in the application, migration command, and
  container configuration.
- Health check names must explicitly identify the target service or resource;
  do not rely on client defaults.

See the [backend guide](../../AGENTS.md) for commands, and the
[REST guide](../port/rest/AGENTS.md) for token and HTTP boundary rules.
