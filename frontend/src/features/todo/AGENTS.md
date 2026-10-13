# Todo feature guide

Follow the [feature guide](../AGENTS.md). The path is defined in [router.tsx](../../router.tsx); planning behavior is in the [backend Todo guide](../../../../backend/internal/application/todo/AGENTS.md).

## Features and page

- `/today`: show today's generated Task/ActionItem work and fixed Schedules with progress; complete/reopen, add/remove, reorder, or hide/restore items.

## Key files

- [pages/Today](../../pages/Today/index.tsx): screen composition.
- [TodayAPIDataProvider](providers/TodayAPIDataProvider/provider.tsx): server Todo query and refresh.
- [TodayActionProvider](providers/TodayActionProvider/provider.tsx): item commands and Query cache coordination.
- [TodayBoard](components/TodayBoard/index.tsx), [todoItems2boardItems.ts](converters/todoItems2boardItems.ts): rendering and API-to-board mapping.

## Change checks

- The backend generates the Todo when that local day's Todo is first created, usually on first read. Later source changes synchronize Schedule rows but never reselect work. Keep frontend logic to presentation and interaction.
- `position` is the only persisted display order. Reordering changes position only; work rows have no planned start/end time, while Schedules retain their actual time.
- Mutations use the current Todo revision header; refresh server state after failures or cross-feature changes. Keep completed work and hidden Schedules visible according to the server snapshot.
- Manual regeneration is a destructive, explicitly confirmed reset: it discards all saved Todo rows and their completion records, manual additions/removals, order, and Schedule visibility settings. Source Task/ActionItem/Schedule completion and actual time remain; completed Tasks and ActionItems are excluded from the rebuilt list. Apply the returned snapshot and invalidate Todo completion metrics after success.

## Verification

- [Today E2E](../../../e2e/today.spec.ts), TodayBoard tests, and TodayActionProvider/revision-header tests.
