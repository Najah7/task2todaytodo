# Allocation feature guide

Follow the [feature guide](../AGENTS.md). Paths are defined in [router.tsx](../../router.tsx); backend behavior is in the [Allocation guide](../../../../backend/internal/application/allocation/AGENTS.md).

## Features and pages

- `/weekly-plan`: inspect Project allocation and sleep windows for a week; `/weekly-plan/edit`: edit weekly Rules and target-minute Plans.
- `/profile`: edit profile sleep settings. Per-day sleep exceptions are edited from the weekly plan.

## Key files

- [AllocationAPIDataProvider](providers/AllocationAPIDataProvider/index.tsx), [AllocationActionProvider](providers/AllocationActionProvider/index.tsx), [AllocationNavigationProvider](providers/AllocationNavigationProvider/index.tsx): week queries, mutations, and URL state.
- [useAllocationPlanForm.ts](hooks/useAllocationPlanForm.ts), [AllocationPlanForm](components/AllocationPlanForm/index.tsx): editable weekly Rules and Plans.
- [WeeklyPlanBoard](components/WeeklyPlanBoard/index.tsx), [DailySleepEditor](components/DailySleepEditor/index.tsx): week display and sleep exceptions.
- [allocationResources2formValues.ts](converters/allocationResources2formValues.ts): server Rules/Plans to form state.

## Change checks

- Weekly Rules and Plans have separate revisions and save outcomes. A partial success must update only the saved part's baseline and keep the failed part editable.
- Keep local dates/timezones and original occurrence identity intact when editing/moving windows; show overlap or paused-rule errors near the relevant UI.
- Project status or allocation changes may alter `/today`; rely on backend overlap and access rules rather than implementing another client policy.

## Verification

- [Allocations E2E](../../../e2e/allocations.spec.ts), `useAllocationPlanForm.test.tsx`, and AllocationActionProvider tests.
