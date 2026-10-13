# Metric guide

Follow the [application guide](../AGENTS.md). Routes are registered in [cmd/api/main.go](../../../cmd/api/main.go).

## Features

- Read the signed-in user's Todo completion by day and period, and current overdue work by Project. This context is read-only and has no domain package.
- Completion accepts 7 or 30 days; ratios use completed/planned item counts, with no ratio when a day has zero planned items. Overdue groups are sorted by count, with five displayed and the rest in `other_count`.
- Completion metrics reflect current Todo rows, not an immutable activity log. Explicitly regenerating today's Todo replaces that day's rows, so its planned and completed counts can decrease or change; source completion and actual minutes remain on their source records.

## Endpoints

- `GET /api/metrics/todo-completion?days=7|30`.
- `GET /api/metrics/overdue-work`.

## Key files

- [usecase/todo_completion.go](usecase/todo_completion.go), [usecase/overdue_work.go](usecase/overdue_work.go): input checks, ratios, sorting, and top-five grouping.
- [repository/repository.go](repository/repository.go): SQL-to-DAO mapping.
- [db/queries/metrics.query.sql](../../../db/queries/metrics.query.sql): item and overdue source selection.

## Change checks

- Keep aggregation definitions in SQL/usecases, not the frontend. Todo/Task/Schedule source or completion changes can alter both metrics.
- Preserve actor scoping, user timezone, no-data ratios, and separate endpoint contracts; the [Metrics page](../../../../frontend/src/features/metrics/AGENTS.md) fetches them independently.

## Verification

- [Metrics integration tests](../../../tests/integration/metric/metrics_integration_test.go) and nearby usecase tests.
