# Shared application guide

Follow the [application guide](../AGENTS.md). This directory holds contracts or pure policy used by multiple business contexts; it owns no HTTP endpoints.

## Why this directory exists

- Contexts cannot import one another. Put a rule or contract here only when multiple contexts actually share it; keep context-owned transitions and persistence with their owners.

## File map

- [capability.go](capability.go): typed resource/action pairs so authorization callers use one permission vocabulary instead of ad hoc strings.
- [capability_test.go](capability_test.go): checks the shared capability constructors.
- [project_lifecycle.go](project_lifecycle.go): narrow Project status/count reconciliation port used by child-work flows without importing Project packages.
- [calendar/recurrence.go](calendar/recurrence.go): pure recurrence, local-time/DST resolution, and interval overlap helpers shared by Schedule and Allocation.
- [calendar/recurrence_test.go](calendar/recurrence_test.go): checks recurrence and timezone edge cases.
- [status/status.go](status/status.go): common Task/Project status catalog and labels; each context still owns its transitions.
- [progress/policy.go](progress/policy.go): common Task percentage calculation used by Task and Project progress reads.
- [progress/policy_test.go](progress/policy_test.go): checks percentage boundary behavior.
- [pagination/pagination.go](pagination/pagination.go): shared page-size limits and `LIMIT pageSize+1` result window; cursor order and ownership remain with each list.
- [pagination/pagination_test.go](pagination/pagination_test.go): checks page-window and empty-result behavior.

## Change checks

- A shared change can affect several contexts: check all call sites and owner-context tests, not only this package's unit tests.
- Keep this package free of business-context, REST, and database dependencies. Do not move a one-context helper here merely because transport or tests reuse it.
- Preserve separate source-of-truth boundaries: shared status is a catalog, not lifecycle policy; shared calendar is calculation, not Schedule or Allocation persistence.

## Verification

- Run the nearby unit tests and relevant [backend integration tests](../../../tests/integration/) for consumers of the changed contract.
