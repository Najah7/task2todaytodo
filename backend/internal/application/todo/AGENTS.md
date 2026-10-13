# Todo guide

Follow the [application guide](../AGENTS.md). Routes are registered in [cmd/api/main.go](../../../cmd/api/main.go).

## Features

- Generate a user's execution plan once when the day's Todo is first created. Free plans use eligible Task/ActionItem work, fixed Schedules, positive Active time windows, priority/urgency, and the existing gap policy while ignoring Project allocations. Pro/Team plans use Project allocations when any occur that day; when none occur, use allocation-free placement.
- Allow an explicit destructive regenerate from current sources. It replaces every Todo row, including completed, hidden, and manually added rows; preserves the Todo identity and advances its revision once. It does not alter source completion or actual-minute state. Free and days with no Project allocation occurrences use allocation-free placement. Pro/Team may request one allocation-free regeneration; this choice applies only to that invocation.
- Keep the selected work and its relative display order stable for the day. Source edits do not reselect, reflow, split, or shorten saved work. Schedule source changes synchronize Schedule rows transactionally while preserving work rows, work timestamps, and relative order.
- Support position-only reorder, manual add/remove, whole-source complete/reopen, and hide/restore for Schedule rows.
- Completed-Task retention and cleanup are deferred; do not infer or implement an automatic deletion policy from this Todo plan work.

## Endpoints

- `GET /api/today-todo`; `POST /api/today-todo:regenerate`; `POST /api/today-todo:reorder`.
- `POST /api/today-todo/action-items/{actionItemId}`, `/api/today-todo/tasks/{taskId}`.
- `POST /api/today-todo/items/{itemId}:complete`, `/api/today-todo/items/{itemId}:reopen`, `/api/today-todo/items/{itemId}:remove`.
- `POST /api/today-todo/schedules/{itemId}:hide`, `/api/today-todo/schedules/{itemId}:restore`.

## Key files

- [domain/planner_generate.go](domain/planner_generate.go): initial whole-item placement in active windows and allocation windows.
- [domain/schedule_insertion.go](domain/schedule_insertion.go): shared position rule for inserting Schedule rows.
- [usecase/get_today.go](usecase/get_today.go), [usecase/generate_today.go](usecase/generate_today.go), [usecase/regenerate_today.go](usecase/regenerate_today.go): initial generation, explicit destructive regeneration, and response snapshot.
- [usecase/reflow.go](usecase/reflow.go): position-only reorder persistence.
- [usecase/sync_schedule_sources.go](usecase/sync_schedule_sources.go): transactional synchronization of changed Schedule sources.
- [repository/repository.go](repository/repository.go): Todo rows and source projections.

## Change checks

- Resolve the user's local day, timezone, source access, and database state before invoking the pure planner. Keep writes and reads enforcing transaction invariants on transaction-bound repositories.
- Manual work may have no internal scheduled time. Reordering changes only `position`; work timestamps remain internal and are not returned by the Todo API.
- Schedule add, restore, and start-time changes use the shared insertion rule. End-time-only and other metadata changes preserve position. Removing a Schedule never changes work selection or timestamps.
- Source changes in [Task](../task/AGENTS.md), [Schedule](../schedule/AGENTS.md), or [Allocation](../allocation/AGENTS.md) must not regenerate an existing Todo. Keep [Metric](../metric/AGENTS.md) semantics consistent with whole-source completion.

## Verification

- [Todo integration tests](../../../tests/integration/todo/) and nearby `domain/*_test.go`, `usecase/*_test.go` cover initial generation, position order, Schedule synchronization, and source changes.
