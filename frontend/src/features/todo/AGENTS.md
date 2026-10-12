# Todo feature guide

Follow the [feature guide](../AGENTS.md). The path is defined in [router.tsx](../../router.tsx); planning behavior is in the [backend Todo guide](../../../../backend/internal/application/todo/AGENTS.md).

## Features and page

- `/today`: show today's generated Task/ActionItem work and fixed Schedules with progress; complete/reopen, add/remove, reorder, regenerate, or hide/restore items.

## Key files

- [pages/Today](../../pages/Today/index.tsx): screen composition.
- [TodayAPIDataProvider](providers/TodayAPIDataProvider/provider.tsx): server Todo query and refresh.
- [TodayActionProvider](providers/TodayActionProvider/provider.tsx): item commands and Query cache coordination.
- [TodayBoard](components/TodayBoard/index.tsx), [todoItems2boardItems.ts](converters/todoItems2boardItems.ts): rendering and API-to-board mapping.

## Change checks

- The backend decides generated order, times, progress, and action capabilities. Keep frontend logic to presentation and interaction.
- Mutations use the current Todo revision header; refresh server state after failures or cross-feature changes. Regeneration is an explicit action.
- Task, Schedule, and Allocation changes can change this screen; keep completed work and hidden Schedules visible according to the server snapshot.

## Verification

- [Today E2E](../../../e2e/today.spec.ts), TodayBoard tests, and TodayActionProvider/revision-header tests.
