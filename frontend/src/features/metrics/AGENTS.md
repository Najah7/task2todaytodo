# Metrics feature guide

Follow the [feature guide](../AGENTS.md). The path is defined in [router.tsx](../../router.tsx); definitions are in the [backend Metric guide](../../../../backend/internal/application/metric/AGENTS.md).

## Features and page

- `/metrics`: show personal Todo completion for the most recent 7 or 30 days and current overdue work by Project. The `days` query parameter selects the period.

## Key files

- [pages/Metrics](../../pages/Metrics/index.tsx): separate completion/overdue queries, URL period, loading/error/refresh states, and response mapping.
- [MetricsDashboard](components/MetricsDashboard/index.tsx), [types.ts](components/MetricsDashboard/types.ts): summary cards, daily bars/table, and overdue groups.

## Change checks

- Use the two generated metrics queries independently. Counts, ratios, and top-five grouping come from the backend; frontend only formats and presents them.
- Distinguish zero planned items (`null` ratio) from 0% completion. Show today's partial progress and retain usable data when a background refresh fails.
- Todo, Task, and Schedule changes can alter these values; do not add another client-side metric definition.

## Verification

- [Metrics page tests](../../pages/Metrics/index.test.tsx) and [MetricsDashboard stories](components/MetricsDashboard/MetricsDashboard.stories.tsx).
