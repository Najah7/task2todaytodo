# Database guide

These rules cover PostgreSQL migrations, sqlc query sources, and generated
code. See the [backend guide](../AGENTS.md) for repository commands and the
[application guide](../internal/application/AGENTS.md) for repository mapping
and business-layer boundaries.

## SQL generation

- `db/queries/` and `sqlc.yml` are sqlc sources. Generated code lives in
  `db/sqlc/`.
- Change query SQL and regenerate sqlc when a query projection or order changes.
  Never hand-edit generated sqlc code.
