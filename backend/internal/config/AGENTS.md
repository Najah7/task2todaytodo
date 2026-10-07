# Runtime configuration guide

These rules cover process configuration in `internal/config` and its use by
`cmd/api`, the application, migrations, and container configuration. Read this
guide when changing runtime configuration in any of those areas.

- The process environment is runtime configuration. An env file works only if
  the launch process loads or exports it.
- `internal/config` owns process environment loading and validation. Pass
  database settings into `internal/application`; application code must not read
  process environment.
- `cmd/api/main.go` creates the REST cursor-token codec from the validated
  `PAGE_TOKEN_KEY` and passes it to handlers.
- Keep database settings consistent in the application, migration command, and
  container configuration.
- Health check names must explicitly identify the target service or resource;
  do not rely on client defaults.

See the [backend guide](../../AGENTS.md) for commands, and the
[REST guide](../port/rest/AGENTS.md) for token and HTTP boundary rules.
