# Progress specification

Task progress is an integer percentage calculated for each response from that
Task's TodoItems only. Schedules do not affect Task progress. Project progress
is the truncated mean of each Task's exposed percentage and each eligible
Schedule occurrence as a separate `0` or `100` value. A recurring Schedule
series contributes one item per eligible occurrence. A Project with no Tasks
and no eligible Schedule occurrences has progress `0`. No progress value is
persisted or accepted as create or update input.

For a Task, eligible work is the set of nondeleted saved TodoItem occurrences,
plus otherwise-unsaved virtual occurrences due today in each recurrence's
timezone. Future virtual occurrences are excluded; unsaved past occurrences
are not reconstructed. Saved occurrences at any date, including future edited
or completed occurrences and saved history whose root was deleted, remain
eligible. A deleted root produces no virtual occurrences. Project Schedule
eligibility follows the same saved-live and local-today rule, computed inside
the Schedule context; future virtual occurrences are excluded.

Occurrence identity is `(resource kind, series_id, occurrence_date)`. A saved
child overrides its root or virtual occurrence at that key. A softdeleted child
suppresses that key rather than exposing the root or virtual occurrence.
Skipped and deleted occurrences do not count; restoring makes the occurrence
eligible again. TodoItems have equal weight within Task progress. In Project
progress, each Task's whole percentage and each eligible Schedule occurrence
have equal weight.

Task progress is `floor(100 * completed / total)`. A non-done task with no
eligible occurrences has progress `0`; a done task always exposes `100`.
Automatic Task completion uses exact TodoItem counts (`total > 0 && completed == total`), not
the rounded percentage.

Explicit task status operations preserve the requested status and recalculate
progress. In particular, reopening a fully completed task leaves it open with
progress `100`. Explicit completion exposes `100` even with unfinished child
work and retains recurrence metadata; existing done-task recurrence
suppression applies.

Child mutations recalculate counts transactionally. Only a mutation that
changes completed or total counts may trigger automatic status behavior. A
non-done task becomes done when exact counts are complete, including while a
recurrence remains active. A done task reopens when the mutation adds unfinished
eligible work. Date rollover and reads never mutate status. Title edits and
reorders with no count change do not trigger status changes.

Projects use the same five statuses and apply the same count-change guard at
the direct child-work level. Each live child Task contributes one eligible
item, completed exactly when its status is `done`; each eligible saved or
today-virtual Schedule occurrence contributes one eligible item and is complete
when that occurrence is complete. A Project becomes done only when a mutation
changes those counts, at least one item is eligible, and all eligible items
are complete. A done Project reopens only when a mutation increases its count
of unfinished eligible direct work. Count-neutral edits do not change Project
status. Explicit Project status changes remain authoritative; a manually
reopened Project can stay open at progress 100, while a done Project displays
progress 100.

Project mutation paths capture work state and reconcile automatic status in the
same transaction. They lock the Project before the child row; moves lock their
old and new Projects in stable ID order. Child lifecycle reconciliation uses
the Project owner's internal usecase contract and does not require the actor
to have `project/update` permission.

A done Project suppresses newly generated virtual recurring Schedule work and
operations that would create an unsaved virtual occurrence. It does not hide or
disable saved Task or Schedule children. Restoring a saved skipped occurrence
is allowed and may reopen the Project if it adds unfinished eligible work.
A trashed Project hides ordinary child reads and writes; restoring the parent
does not change child deletion state.
