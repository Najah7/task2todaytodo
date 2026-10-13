# Database guide

These rules cover PostgreSQL migrations, sqlc query sources, and generated
code. See the [backend guide](../AGENTS.md) for repository commands and the
[application guide](../internal/application/AGENTS.md) for repository mapping
and business-layer boundaries.

## Migrations

- Organize migrations by domain or feature, with one up/down pair per unit.
- While the product is pre-release, update the existing initial migration
  definitions when a schema design changes. Do not append migration history
  that renames, alters, or backfills an initial schema which has not shipped.
- Keep each down migration consistent with its up migration and reverse object
  dependencies when dropping schema.
- Migration SQL in `db/migrations/` is the schema source of truth. Every schema
  change must update the matching `docs/er/*.mmd` diagram. For a new migration,
  add its diagram and list the pair in `docs/er/README.md`; keep columns,
  constraints, and relationships consistent with the migration.
- Rebuild an existing development database separately when migration history
  changes. Do not modify it as part of migration authoring.

## SQL generation

- `db/queries/` and `sqlc.yml` are sqlc sources. Generated code lives in
  `db/sqlc/`.
- Change query SQL and regenerate sqlc when a query projection or order changes.
  Never hand-edit generated sqlc code.
