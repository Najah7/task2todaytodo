# Schedule feature guide

Follow the [feature guide](../AGENTS.md). Paths are defined in [router.tsx](../../router.tsx); backend behavior is in the [Schedule guide](../../../../backend/internal/application/schedule/AGENTS.md).

## Features and pages

- `/schedules`, `/schedules/new`, `/schedules/:ruleId/:occurrenceDate/edit`: calendar/list, create, and edit recurring or one-off Schedules.
- Schedule occurrences also appear in `/inbox` and `/today`; those Pages own their interactions.

## Key files

- [ScheduleAPIDataProvider](providers/ScheduleAPIDataProvider/index.tsx), [ScheduleActionProvider](providers/ScheduleActionProvider/index.tsx), [ScheduleNavigationProvider](providers/ScheduleNavigationProvider/index.tsx): occurrence data, mutations, and URL state.
- [ScheduleCalendar](components/ScheduleCalendar/index.tsx), [ScheduleForm](components/ScheduleForm/index.tsx): calendar and shared form.
- [useScheduleForms.ts](hooks/useScheduleForms.ts), [converters](converters/): draft lifecycle, scope-specific requests, and event/form mapping.

## Change checks

- Address edits by rule ID and original occurrence date. `current` and `following` scopes have different API operations; keep occurrence and rule revisions separate.
- Preserve virtual occurrence revision `0` and conflict reload. Display timezone/all-day values from the returned occurrence/rule without duplicating recurrence rules in the client.
- Schedule changes synchronize today's fixed-time Schedule rows without reselecting or reordering work. Capability flags govern edit/delete/following actions.

## Verification

- [Schedules E2E](../../../e2e/schedules.spec.ts) and colocated Schedule form, converter, and Provider tests.
