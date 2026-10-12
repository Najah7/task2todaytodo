# Task guide

Follow the [application guide](../AGENTS.md). Routes are registered in [cmd/api/main.go](../../../cmd/api/main.go).

## Features

- Manage Tasks and their ActionItems, status, priority, estimates, assignees, revisions, and progress. Tasks may belong to a Project or stand alone.
- Project Task routes list, create, add, and remove Tasks in a Project; ActionItem operations are separate from Task writes.

## Endpoints

- `GET /api/tasks`, `/api/tasks/{id}`, `/api/tasks/{id}/revisions`, `/api/tasks/{id}/assignees`, `/api/projects/{id}/tasks`, `/api/tasks/{taskId}/action-items`.
- `POST /api/tasks`, `/api/tasks/{id}:start`, `/api/tasks/{id}:hold`, `/api/tasks/{id}:wait`, `/api/tasks/{id}:complete`, `/api/tasks/{id}:reopen`.
- `PATCH /api/tasks/{id}`, `/api/tasks/{id}/assignees`; `DELETE /api/tasks/{id}`.
- `POST /api/projects/{id}/tasks`, `/api/projects/{id}/tasks:add`, `/api/projects/{id}/tasks:remove`.
- `POST /api/tasks/{taskId}/action-items`, `/api/tasks/{taskId}/action-items/{id}:complete`, `/api/tasks/{taskId}/action-items/{id}:reopen`, `/api/tasks/{taskId}/action-items/{id}:reorder`.
- `PATCH`, `DELETE /api/tasks/{taskId}/action-items/{id}`.

## Key files

- [domain/task.go](domain/task.go), [domain/action_item.go](domain/action_item.go): Task and ActionItem state and invariants.
- [usecase/update_task.go](usecase/update_task.go), [usecase/create_action_item.go](usecase/create_action_item.go): revision, Project access, and Todo-regeneration checks.
- [usecase/progress.go](usecase/progress.go), [repository/tasks.go](repository/tasks.go): progress orchestration and Task persistence.
- [repository/project_assignees.go](repository/project_assignees.go): Project assignment data access.

## Change checks

- Keep Task and ActionItem writes as separate operations; preserve revision checks, assignment permissions, and regeneration confirmation when a saved Todo is affected.
- Project member removal reaches Task-owned reassignment through a narrow transaction-bound port. Log status changes only after persistence succeeds.
- Changes to Task source or completion semantics affect [Todo](../todo/AGENTS.md), [Inbox](../inbox/AGENTS.md), [Metric](../metric/AGENTS.md), and the [Task UI](../../../../frontend/src/features/task/AGENTS.md).

## Verification

- [Task integration tests](../../../tests/integration/task/) cover sharing, progress, pagination, assignments, and ActionItem behavior; [REST task tests](../../../tests/integration/rest/) cover editing and list flows.
