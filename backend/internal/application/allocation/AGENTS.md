# Allocation guide

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

`POST /admin/jobs/prefill-allocations:run` uses the shared admin operation
runner and is invoked by external infrastructure. Each user's current and next
two local weeks are materialized with a single UTC run cutoff. The operation is
idempotent and checkpoints progress per user.
