# Schedule guide

Follow the [application guide](../AGENTS.md) for architecture and boundaries.
These product rules govern Schedule behavior. Keep this guide aligned with
approved decisions.

## Features

- Create one-off or recurring Schedule rules, project virtual occurrences, and persist individual occurrence changes. Support individual or future-scoped edits, revisions, completion, skipping, restoration, and backfill.
- Preserve fixed Schedule times for Todo planning. Project completion pauses recurring rules; reopening resumes future cadence.

## Endpoints

- `POST /api/schedule-rules`, `/api/projects/{id}/schedules`; `GET /api/schedule-rules/{id}`, `/api/schedule-rules/{id}/revisions`, `/api/schedule-rules/{id}/assignees`, `/api/schedules`, `/api/projects/{id}/schedules`.
- `PATCH /api/schedule-rules/{id}`; `DELETE /api/schedule-rules/{id}/following/{occurrence_date}`.
- `GET /api/schedules/{rule_id}/{occurrence_date}`, `/api/schedules/{rule_id}/{occurrence_date}/revisions`; `PATCH`, `DELETE /api/schedules/{rule_id}/{occurrence_date}`.
- `POST /api/schedules/{rule_id}/{occurrence_date}:complete`, `/api/schedules/{rule_id}/{occurrence_date}:reopen`, `/api/schedules/{rule_id}/{occurrence_date}:skip`, `/api/schedules/{rule_id}/{occurrence_date}:restore`, `/api/schedules/{rule_id}/{occurrence_date}:reschedule`.
- Backfill trigger: `POST /api/admin/jobs/backfill-schedules:run` (Admin-owned). Routes are registered in [cmd/api/main.go](../../../cmd/api/main.go).

## Key files

- [domain/recurrence.go](domain/recurrence.go), [domain/rule.go](domain/rule.go): recurrence and rule behavior.
- [usecase/resolve_occurrence.go](usecase/resolve_occurrence.go), [usecase/update_rule.go](usecase/update_rule.go): virtual occurrence resolution and future-effective changes.
- [usecase/update_occurrence.go](usecase/update_occurrence.go), [usecase/backfill.go](usecase/backfill.go): individual snapshots and materialization.
- [repository/rules.go](repository/rules.go), [repository/schedules.go](repository/schedules.go): persisted rules, exceptions, and projections.

## Change checks

- Address an occurrence by rule ID and original local date; a move does not change that identity. Individual edits update the full exception and concrete projection in one transaction.
- Keep recurrence in the rule's IANA timezone, including DST and all-day boundaries. Future-effective edits must preserve prior history and saved exceptions.
- Backfill must not overwrite exceptions or trigger Project status changes; Project pause/resume affects [Todo](../todo/AGENTS.md), [Inbox](../inbox/AGENTS.md), and the [Schedule UI](../../../../frontend/src/features/schedule/AGENTS.md).

## Verification

- [Schedule integration tests](../../../tests/integration/schedule/rules_integration_test.go), [REST history flow](../../../tests/integration/rest/schedule_history_flow_integration_test.go), and nearby domain/usecase tests.

## Behavior reference

- Follow Google Calendar's user-facing Schedule behavior wherever possible.
  Explicit product decisions below take precedence. Clarify material differences
  before implementing them; do not assume Google's API matches its UI behavior.
- Keep recurrence rules and all-day boundary rules in the backend.
- Log occurrence complete, reopen, skip, restore, and reschedule operations after successful persistence.

## Rules and schedules

- `schedule_rules` stores stable source identities;
  `schedule_rule_revisions` stores effective definitions. `schedule_exceptions`
  is the authoritative full snapshot of an individually changed occurrence;
  `schedules` is its synchronized concrete projection for stable schedule IDs,
  history, Todo foreign keys, and progress reads. `schedule_revisions` audits
  projection changes.
- Create both one-off and recurring definitions through `/schedule-rules`.
  A one-off creation atomically creates one rule and one schedule.
- Recurring creation does not immediately persist the first occurrence. Reads
  project unsaved occurrences, including past occurrences awaiting the job.
  Editing, completing, or skipping an individual occurrence materializes it.
- Every individual edit or state change writes the complete exception snapshot
  and synchronizes its schedule projection in the same transaction. No
  individual write may update only `schedules`. Backfill skips every existing
  exception by its original `rule_id` and `occurrence_date`, including moved,
  skipped, deleted, and out-of-range occurrences.
- Address occurrences by `rule_id` and their original local
  `occurrence_date`. Moving an occurrence never changes this identity.
- Virtual occurrences have no persisted ID and revision `0`; their first
  mutation accepts `If-Match: "0"`. Saved occurrences use positive revisions.
- Project and assignee changes support the same individual/following scopes as
  title and time changes. Preserve their values on earlier occurrences.
- `/schedules` serves individual occurrences. Lists require UTC `from` and `to`,
  return stored and virtual occurrences overlapping the range, and never write.
  Personal lists include occurrences assigned to the caller; Project lists
  include all occurrences the caller may read in that Project.

## Time and recurrence

- Rules retain local start/end datetimes and an explicit IANA timezone.
  Concrete schedules store and return UTC instants. Compute recurrence in the
  rule's timezone so local clock times remain stable across DST changes.
- Allow timezone changes through future-effective rule revisions, preserving
  earlier occurrences and saved individual exceptions. Changing the user's
  timezone affects display conversion, never existing rule definitions.
- Individual moves may change the local date. Future-scoped datetime changes
  must retain the selected occurrence's local start date; moving that start date
  for an entire future recurrence is outside this iteration.
- Skip occurrences with nonexistent local times during spring-forward. For
  ambiguous fall-back times, select the earlier instant and generate only once.
- Use `timezone_master` for allowed IANA region names and `UTC`, shared by user
  settings and rules. Expose choices through `GET /timezones`; validate
  membership in both the API and database. Never store a fixed UTC offset as
  the timezone's authority; resolve offsets for each datetime.
- Support daily, weekly with selected weekdays, monthly by day of month, and
  yearly recurrence, each with a positive interval.
- Clamp missing monthly dates to the month's last day while retaining the
  original anchor: January 31, February 28/29, March 31. Yearly February 29
  similarly becomes February 28 in non-leap years.
- Keep the nominal monthly/yearly anchor separate from the interval's start
  date. Same-frequency changes retain the anchor; frequency changes reset it.
- End recurrence either without a limit or on an inclusive local start date.
  Count-based endings and ordinal-weekday monthly rules are outside this scope.

## Changes and history

- “This and following” creates a new effective rule revision. Preserve
  earlier definitions and history; backfill uses the revision effective for
  each occurrence.
- Allow this scope only when the selected occurrence's original start is still
  in the future. Started occurrences remain individually editable.
- Rule changes preserve individually saved edits, completions, and skips,
  including future ones.
- A new "this and following" change supersedes previously planned rule
  revisions in that range. Retain their audit history without applying their
  future settings; preserve saved individual exceptions as above.
- Support one-off to recurring and recurring to one-off changes for future
  occurrences. A one-off can become the recurrence anchor; switching to one-off
  retains the selected occurrence as the last base occurrence. Preserve past
  history and saved individual exceptions.
- Deleting this and following cancels the selected occurrence and everything
  after it, including saved future exceptions. Preserve earlier history and
  stop subsequent generation.

## Project completion

- Pause associated recurring rules when a Project becomes done, through
  either manual or automatic completion. Share the completion workflow and
  commit Project completion and pause intervals in the same transaction.
- Automatically resume only rules paused by Project completion when the
  Project reopens, through a shared transactional workflow. Preserve user-ended,
  deleted, and expired recurrence definitions.
- Preserve the original recurrence cadence on resume. Do not generate occurrences
  whose original start falls within the Project's done interval; resume from
  the reopening time without backfilling that interval.
- Project progress and automatic completion/reopening count only persisted,
  nondeleted, nonskipped Schedules, including saved future occurrences. Virtual
  occurrences do not count before they are materialized. A Project may become
  done while a rule still has ungenerated occurrences; the done transition
  pauses its recurring rules.
- Backfill materializes eligible occurrences but never triggers Project status
  transitions or pause/resume workflows. Its single UTC cutoff is captured when
  the job starts.
- Give Projects the same status action endpoints as Tasks: `:start`, `:hold`,
  `:wait`, `:complete`, and `:reopen` under `/projects/{id}`.

## All-day occurrences and calendar filters

- `all_day` defaults to `false`. All-day rule boundaries are local midnight
  timestamps; `local_end` is the exclusive local date. Resolve each boundary in
  the rule timezone so multi-day spans retain calendar-day length across
  daylight-saving changes. Concrete occurrences keep UTC instants and their
  timezone, and revisions copy `all_day`.
- Individual occurrence updates may toggle `all_day`; timezone remains
  read-only. Explicit all-day boundaries must be local midnight.
- Schedule list routes accept `include_skipped` (default `false`). Deleted
  occurrences remain hidden; restore only clears the skipped state. Bind this
  filter into cursor scope.
- Occurrence responses expose server-computed per-occurrence edit, delete, skip,
  assignment, and following capabilities. Following capabilities use the
  effective rule's original start for the occurrence date and are
  available only before that start.

## Backfill job

- Expose `POST /admin/jobs/backfill-schedules:run` in the same API codebase,
  triggered by an external scheduler using a dedicated management user's PAT.
- Require `admin_job:run`, granted only to the system-wide `sysadmin` role.
  Keep this role separate from Project roles; Project member operations cannot
  assign it. Ordinary users and Project administrators cannot run admin jobs.
  Provision system-role assignments operationally, without a public grant API.
- Run asynchronously using the Long-running Operations (LRO) pattern. Persist
  the operation, start background execution directly when `:run` is called, and
  return `202 Accepted` with the operation. Execution outlives the HTTP request.
  Assume a continuously running API container with CPU outside requests.
- Expose read-only `GET /admin/operations` and `GET /admin/operations/{id}` for
  status, progress, results, and errors. Do not expose wait, cancel, update, or
  delete operations in this iteration. Retain committed work on failure so
  subsequent runs can resume pending work.
- Mark interrupted operations as failed once interruption is detected. A later
  `:run` creates a new operation and continues from persisted checkpoints;
  preserve the failed operation as history rather than automatically resuming it.
- Allow only one active `backfill-schedules` operation across API instances.
  Repeated `:run` calls return the active operation without launching another
  execution. Calls after completion create a new operation.
- Capture one UTC cutoff per run. Materialize recurring occurrences whose
  original scheduled start precedes that cutoff, independent of local midnight.
- Catch up from the last successfully processed position. Persist generation
  and checkpoint advancement atomically per rule. Retries must not create
  duplicates or overwrite saved individual occurrences.
- If a new recurring rule starts in the past, backfill from its specified
  start rather than its creation time, subject to effective revisions and Project
  done intervals.
