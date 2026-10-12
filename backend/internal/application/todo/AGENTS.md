# Todo guide

Follow the [application guide](../AGENTS.md). Routes are registered in [cmd/api/main.go](../../../cmd/api/main.go).

## Features

- Build today's execution plan from Task/ActionItem work, fixed Schedules, sleep, and Project allocations. Place eligible work in available time while keeping Schedule times fixed.
- Return generated frames and progress; support regeneration, manual ordering, adding work, completing/reopening work, and hiding/restoring Schedules.

## Endpoints

- `GET /api/today-todo`; `POST /api/today-todo:regenerate`, `/api/today-todo:reorder`.
- `POST /api/today-todo/action-items/{actionItemId}`, `/api/today-todo/tasks/{taskId}`.
- `POST /api/today-todo/items/{itemId}:complete`, `/api/today-todo/items/{itemId}:reopen`, `/api/today-todo/items/{itemId}:remove`.
- `POST /api/today-todo/schedules/{itemId}:hide`, `/api/today-todo/schedules/{itemId}:restore`.

## Key files

- [domain/planner_generate.go](domain/planner_generate.go), [domain/planner_reflow.go](domain/planner_reflow.go): automatic planning and manual reflow.
- [usecase/get_today.go](usecase/get_today.go): creates today's Todo when absent and builds the response snapshot.
- [usecase/regenerate_today.go](usecase/regenerate_today.go), [usecase/reflow.go](usecase/reflow.go): regeneration and reorder persistence.
- [repository/repository.go](repository/repository.go): Todo frames and source projections.

## Change checks

- Resolve user-local day, timezone, source access, and database state before calling the pure planner. Keep writes and reads enforcing transaction invariants on transaction-bound repositories.
- Regeneration must preserve explicit completed frames and hidden Schedule state; reflow keeps fixed Schedule anchors.
- Source changes in [Task](../task/AGENTS.md), [Schedule](../schedule/AGENTS.md), or [Allocation](../allocation/AGENTS.md) may change the generated plan and [Metric](../metric/AGENTS.md) results.

## Verification

- [Todo integration tests](../../../tests/integration/todo/) and nearby `domain/planner_test.go`, `usecase/*_test.go` cover generation, reflow, and source changes.
