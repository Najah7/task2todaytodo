# Task feature guide

Follow the [feature guide](../AGENTS.md). Paths are defined in [router.tsx](../../router.tsx); backend behavior is in the [Task guide](../../../../backend/internal/application/task/AGENTS.md).

## Features and pages

- `/tasks`, `/tasks/new`, `/tasks/:id/edit`: list, create, edit, assign, change status, and delete Tasks and their ActionItems.
- Task and ActionItem rows also appear in `/inbox` and `/today`; those Pages own their interactions.

## Key files

- [TaskAPIDataProvider](providers/TaskAPIDataProvider/index.tsx), [TaskActionProvider](providers/TaskActionProvider/index.tsx), [TaskNavigationProvider](providers/TaskNavigationProvider/index.tsx): server data, mutations, and URL state.
- [TaskForm](components/TaskForm/index.tsx), [useTaskCreateForm.ts](hooks/useTaskCreateForm.ts), [useTaskEditForm.ts](hooks/useTaskEditForm.ts): shared form and draft lifecycle.
- [saveActionItems.ts](providers/TaskActionProvider/saveActionItems.ts), [applyTaskFormSaveResult.ts](hooks/applyTaskFormSaveResult.ts): independent child saves and partial results.
- [converters](converters/): API/form mappings and list view models.

## Change checks

- Task and ActionItem saves are individual requests; preserve row-level errors, successful-field resets, and retry of only unsaved operations.
- After partial creation, retain the created Task ID to avoid a duplicate on retry. Keep revision and Todo-regeneration confirmation handling in the Action Provider/form flow.
- Changes to status, progress, or assignment may affect `/inbox`, `/today`, and `/metrics`; use server capabilities for available actions.

## Verification

- [Tasks E2E](../../../e2e/tasks.spec.ts) plus colocated Task form, converter, and Action Provider tests.
