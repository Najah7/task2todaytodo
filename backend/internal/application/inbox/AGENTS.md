# Inbox guide

Follow the [application guide](../AGENTS.md). The route is registered in [cmd/api/main.go](../../../cmd/api/main.go).

## Features

- Read a combined Task/Schedule Inbox with type, Project, and title filters, counts, and cursor pagination. It does not own Task or Schedule mutations.
- Show active Tasks and uncompleted, unskipped Schedule occurrences in the user's local seven-day window.

## Endpoint

- `GET /api/inbox` (filters and page token). Inbox quick actions call Task, Schedule, or Todo endpoints owned by those contexts.

## Key files

- [usecase/list.go](usecase/list.go): snapshot, window, filtering, sorting, counts, and pagination.
- [dao/item.go](dao/item.go): combined read item shape.
- [repository/metadata.go](repository/metadata.go): source metadata projection.
- [application/inbox_usecase.go](../inbox_usecase.go): cross-context candidate wiring.

## Change checks

- A page snapshot includes UTC `asOf`, timezone, and local today; subsequent pages must use the same window and cursor scope.
- Keep Task and Schedule ownership/visibility rules in their source contexts; Inbox combines readable, nondeleted candidates only.
- Changes to candidate fields or filters affect [frontend Inbox](../../../../frontend/src/features/inbox/AGENTS.md) and pagination tests.

## Verification

- [Inbox integration tests](../../../tests/integration/inbox/inbox_integration_test.go) and [list usecase tests](usecase/list_test.go).
