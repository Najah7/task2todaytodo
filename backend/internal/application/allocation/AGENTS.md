# Allocation guide

Follow the [application guide](../AGENTS.md). Routes are registered in [cmd/api/main.go](../../../cmd/api/main.go).

## Features

- Define weekly and one-off Project allocation windows, materialize their occurrences, and serve weekly plans. Provide sleep windows from profile settings and per-day exceptions.
- Weekly Rules and target-minute Plans are independent resources with separate revisions; Project completion pauses Rules and reopening resumes their cadence.

## Endpoints

- `GET /api/allocation-weeks/{week_start}`, `/api/allocation-rules`, `/api/allocation-plans`; `PUT /api/allocation-rules`, `/api/allocation-plans`.
- `POST /api/allocations`; `PATCH/DELETE /api/allocations/{rule_id}/{occurrence_date}`.
- `PUT/DELETE /api/daily-sleep-exceptions/{local_date}`; related Auth-owned `PATCH /api/users/me/sleep`.
- Prefill trigger: `POST /api/admin/jobs/prefill-allocations:run` (Admin-owned).

## Key files

- [domain/interval.go](domain/interval.go), [domain/sleep.go](domain/sleep.go): overlap and sleep-window rules.
- [usecase/put_rules.go](usecase/put_rules.go), [usecase/put_plans.go](usecase/put_plans.go): separate weekly resource writes.
- [usecase/get_week.go](usecase/get_week.go), [usecase/prefill.go](usecase/prefill.go): weekly projection and materialization.
- [repository/repository.go](repository/repository.go), [usecase/project_lifecycle.go](usecase/project_lifecycle.go): persistence and Project pause/resume coordination.

## Change checks

- Preserve ownership and Project read-access checks; changing Rules must not change Plan revisions or hidden rows, and vice versa.
- Keep rule timezone and original occurrence identity stable; adjacent windows are valid, overlaps are not. Sleep remains a projection, not materialized.
- Project completion/reopen must update allocation pause state in the same transaction; changes affect [Todo](../todo/AGENTS.md) and the [Allocation UI](../../../../frontend/src/features/allocation/AGENTS.md).

## Verification

- [Allocation integration tests](../../../tests/integration/allocation/) cover HTTP flows, Rules/Plans revisions, and concurrency.

## Detailed rules

Project time allocations are user-owned rules and concrete execution frames.
Only the owner may read or edit their allocations. Project read access gates
whether that owner may read or create their own allocations in the Project;
Project edit and Schedule permissions do not grant access to another user's
allocation. Losing Project read access hides and stops that owner's generation.

Rules retain their IANA timezone. Weekly weekday values are 0=Sunday through
6=Saturday. Weekly slots and one-off slots use shared calendar recurrence
helpers. Moving an occurrence preserves its original rule
and occurrence date. Saved exceptions are full snapshots. Allocation and sleep
windows cannot overlap; allocation windows cannot overlap each other. Adjacent
windows are valid. Conflicts never move or shorten other windows automatically.

Sleep settings consist of current profile wake and bedtime values plus optional
per-date full-pair exceptions. Sleep is a read projection, not materialized.
Dates identify the waking day. A bedtime at or before wake time ends on the next
local date. Basic profile changes recalculate virtual sleep days; exception
timezone and values remain as saved.

Project completion pauses its allocation rules in the Project transaction.
Reopening resumes original cadence at reopening time. Rules with conflicts stay
paused and expose a reason until a later edit resolves the conflict. Allocations
never change Project progress or status.

The current weekly configuration is exposed as two independent resources:
`/allocation-rules` replaces only visible weekly Rules, and `/allocation-plans`
replaces only explicitly saved weekly target minutes. Each collection has its
own opaque content revision and `If-Match` value. Changing Rules does not
change the plan revision, and changing plans does not change the Rules
revision. Rules writes preserve Plans, one-off Rules, past and started frames,
and concrete exceptions. Plans writes preserve Rules, occurrences, and
exceptions. Both writes preserve hidden or unreadable rows.

`POST /api/admin/jobs/prefill-allocations:run` uses the shared admin operation
runner and is invoked by external infrastructure. Each user's current and next
two local weeks are materialized with a single UTC run cutoff. The operation is
idempotent and checkpoints progress per user.
