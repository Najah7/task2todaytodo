# Schedule guide

Follow the [application guide](../AGENTS.md) for architecture and boundaries.
These product rules govern Schedule behavior. Keep this guide aligned with
approved decisions.

## Behavior reference

- Follow Google Calendar's user-facing Schedule behavior wherever possible.
  Explicit product decisions below take precedence. Clarify material differences
  before implementing them; do not assume Google's API matches its UI behavior.
- Keep recurrence rules and all-day boundary rules in the backend.

## Blueprints and schedules

- `schedule_blueprints` stores stable source identities;
  `schedule_blueprint_revisions` stores effective definitions. `schedules`
  stores concrete occurrences, with audit in `schedule_revisions`.
- Create both one-off and recurring definitions through `/schedule-blueprints`.
  A one-off creation atomically creates one blueprint and one schedule.
- Recurring creation does not immediately persist the first occurrence. Reads
  project unsaved occurrences, including past occurrences awaiting the job.
  Editing, completing, or skipping an individual occurrence materializes it.
- Address occurrences by `blueprint_id` and their original local
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

- Blueprints retain local start/end datetimes and an explicit IANA timezone.
  Concrete schedules store and return UTC instants. Compute recurrence in the
  blueprint's timezone so local clock times remain stable across DST changes.
- Allow timezone changes through future-effective blueprint revisions, preserving
  earlier occurrences and saved individual exceptions. Changing the user's
  timezone affects display conversion, never existing blueprint definitions.
- Individual moves may change the local date. Future-scoped datetime changes
  must retain the selected occurrence's local start date; moving that start date
  for an entire future recurrence is outside this iteration.
- Skip occurrences with nonexistent local times during spring-forward. For
  ambiguous fall-back times, select the earlier instant and generate only once.
- Use `timezone_master` for allowed IANA region names and `UTC`, shared by user
  settings and blueprints. Expose choices through `GET /timezones`; validate
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

- “This and following” creates a new effective blueprint revision. Preserve
  earlier definitions and history; backfill uses the revision effective for
  each occurrence.
- Allow this scope only when the selected occurrence's original start is still
  in the future. Started occurrences remain individually editable.
- Blueprint changes preserve individually saved edits, completions, and skips,
  including future ones.
- A new "this and following" change supersedes previously planned blueprint
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

- Pause associated recurring blueprints when a Project becomes done, through
  either manual or automatic completion. Share the completion workflow and
  commit Project completion and pause intervals in the same transaction.
- Automatically resume only blueprints paused by Project completion when the
  Project reopens, through a shared transactional workflow. Preserve user-ended,
  deleted, and expired recurrence definitions.
- Preserve the original recurrence cadence on resume. Do not generate occurrences
  whose original start falls within the Project's done interval; resume from
  the reopening time without backfilling that interval.
- Project progress and automatic completion/reopening count only persisted,
  nondeleted, nonskipped Schedules, including saved future occurrences. Virtual
  occurrences do not count before they are materialized. A Project may become
  done while a blueprint still has ungenerated occurrences; the done transition
  pauses its recurring blueprints.
- Backfill materializes eligible occurrences but never triggers Project status
  transitions or pause/resume workflows. Its single UTC cutoff is captured when
  the job starts.
- Give Projects the same status action endpoints as Tasks: `:start`, `:hold`,
  `:wait`, `:complete`, and `:reopen` under `/projects/{id}`.

## All-day occurrences and calendar filters

- `all_day` defaults to `false`. All-day blueprint boundaries are local midnight
  timestamps; `local_end` is the exclusive local date. Resolve each boundary in
  the blueprint timezone so multi-day spans retain calendar-day length across
  daylight-saving changes. Concrete occurrences keep UTC instants and their
  timezone, and revisions copy `all_day`.
- Individual occurrence updates may toggle `all_day`; timezone remains
  read-only. Explicit all-day boundaries must be local midnight.
- Schedule list routes accept `include_skipped` (default `false`). Deleted
  occurrences remain hidden; restore only clears the skipped state. Bind this
  filter into cursor scope.
- Occurrence responses expose server-computed per-occurrence edit, delete, skip,
  assignment, and following capabilities. Following capabilities use the
  effective blueprint's original start for the occurrence date and are
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
  and checkpoint advancement atomically per blueprint. Retries must not create
  duplicates or overwrite saved individual occurrences.
- If a new recurring blueprint starts in the past, backfill from its specified
  start rather than its creation time, subject to effective revisions and Project
  done intervals.
