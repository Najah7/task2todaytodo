# Current recurrence design

This note is the current reference for recurrence behavior. It supersedes
recurrence details in older planning documents where they differ.

## Source and generation

- ActionItem recurrence is rooted in `action_items`; Schedule recurrence is rooted
  in `schedules`. Each context owns its own occurrences and revisions. A Task
  does not own or import Schedules.
- `repeat_state` is `one_off`, `active`, or `stopped`. Active weekday recurrence
  uses `frequency_anchor_date`, `interval_weeks`, and the shared weekday catalog.
- Lists generate virtual occurrences from the user's local today onward, as
  needed to fill the requested page. The system does not materialize a rolling
  window.
- A virtual occurrence uses the stable response ID
  `TASK2TODAYTODO000000000000`. `series_id` remains the root ID. The root
  occurrence and saved occurrence overrides keep their real IDs.
- A saved child row represents an occurrence override, including an individual
  edit or completion. Skipping records a skipped override; restoring removes a
  skip-only child or clears the deleted marker on an edited child.

## Updates and occurrence identity

- Frequency changes apply immediately to the root. Weekdays select matching
  days, and `interval_weeks` sets the number of weeks between matching weeks.
  Setting `interval_weeks` to `0` stops future generation. Use the resource's
  frequency endpoint to change weekdays.
- `PATCH` with `scope=future` updates the root template immediately and does
  not need `occurrence_date`. For ActionItems, `due_date` is rejected for future
  scope, including explicit `null`; use the frequency endpoint to change the
  recurring weekday. Current-scope edits can change an occurrence's due date.
- `occurrence_date` is the stable date identifying a series occurrence. An
  individual ActionItem edit to `due_date` or Schedule edit to `start_at` may
  cause actual date/time to differ from that identity date.
- Schedule rescheduling still identifies the target occurrence with
  `occurrence_date` and validates the requested start date against that
  selection.
- Schedule changes, including recurrence settings, create Schedule history
  revisions. Tag assignment changes are separate and do not create revisions.
