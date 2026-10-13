# Allocation feature guide

Follow the [feature guide](../AGENTS.md). Paths are defined in [router.tsx](../../router.tsx); backend behavior is in the [Allocation guide](../../../../backend/internal/application/allocation/AGENTS.md).

## Features and pages

- `/weekly-plan`: inspect Project allocations and positive 作業可能時間 ranges for a week; `/weekly-plan/edit`: edit weekly Rules and target-minute Plans.
- `/profile`: edit the default 作業可能時間. Per-day overrides are edited from the weekly plan.
- Free users can view and edit active-time settings and daily overrides; weekly allocation data and allocation editing are available only to Pro/Team. The backend owns enforcement, while this UI hides gated data and links to Billing.

## Key files

- [AllocationAPIDataProvider](providers/AllocationAPIDataProvider/index.tsx), [AllocationActionProvider](providers/AllocationActionProvider/index.tsx), [AllocationNavigationProvider](providers/AllocationNavigationProvider/index.tsx): week queries, mutations, and URL state.
- [useAllocationPlanForm.ts](hooks/useAllocationPlanForm.ts), [AllocationPlanForm](components/AllocationPlanForm/index.tsx): editable weekly Rules and Plans.
- [WeeklyPlanBoard](components/WeeklyPlanBoard/index.tsx), [DailyActiveTimeEditor](components/DailyActiveTimeEditor/index.tsx): week display and per-day active-time overrides.
- [allocationResources2formValues.ts](converters/allocationResources2formValues.ts): server Rules/Plans to form state.

## Change checks

- Weekly Rules and Plans have separate revisions and save outcomes. A partial success must update only the saved part's baseline and keep the failed part editable.
- Keep local dates/timezones and original occurrence identity intact when editing/moving windows; show allocation overlap or 作業可能時間 errors near the relevant UI.
- Project status and allocation rules inform initial Todo generation only; they do not reselect work in a saved Todo. Rely on backend overlap and access rules rather than implementing another client policy.

## Verification

- [Allocations E2E](../../../e2e/allocations.spec.ts), `useAllocationPlanForm.test.tsx`, and AllocationActionProvider tests.
