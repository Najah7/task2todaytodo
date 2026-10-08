# Project sharing and revision API

Sharing is controlled by Project memberships. Each membership grants one
global, service-managed role: `viewer`, `editor`, or `admin`. Project owners
have full authority without a membership entry; the owner cannot be added,
removed, or demoted through the members API. Assignment to a Task or Schedule
does not grant Project access.

## Authorization model

Policies are stored in `managed_resources`, `permissions`, and
`role_permissions`. Each permission identifies one resource, one CRUD action,
and an `allow` or `deny` effect. A matching deny overrides any matching allow;
missing allows deny by default. Owners bypass role evaluation. Roles and policy
rows are global and service-managed. The API exposes read-only role and
permission catalogs at `GET /api/roles` and `GET /api/permissions`; it does not
expose role or policy mutation.

Seeded grants:

| Role | Grants |
| --- | --- |
| `viewer` | Read projects, tasks, todo items, and schedules |
| `editor` | Viewer grants; update projects; create and update tasks, todo items, and schedules; update Task and Schedule assignment and recurrence occurrences |
| `admin` | All seeded resource actions, including delete and project-member management; read deleted-resource history |

The resource catalog includes `project`, `task`, `todo_item`, `schedule`,
`task_assignment`, `schedule_assignment`, `occurrence`, `project_member`, and
`deleted_history`. Applications pass resource/action pairs to database policy
checks, so changing role grants changes authorization without role-name
conditionals.

## Sharing and resource routes

All routes below use the `/api` prefix and require bearer authentication.

| Route | Behavior |
| --- | --- |
| `GET /projects` | Lists projects owned by the caller or shared with current read access |
| `GET /projects/options` | Returns the shared Project type, priority, and status catalogs |
| `GET /projects/{id}` | Reads project details and calculated progress |
| `PATCH /projects/{id}/status` | Changes only Project status; requires the current Project ETag in `If-Match` |
| `POST /projects/{id}/restore` | Restores a trashed Project; requires delete permission and the current Project ETag |
| `GET /projects/{id}/tasks` | Lists visible Tasks in that Project, regardless of assignee |
| `GET /projects/{id}/schedules` | Lists visible Schedules in that Project, regardless of assignee |
| `POST /projects/{id}/schedules` | Creates a Schedule owned by the Project owner and initially assigned to that owner |
| `GET /projects/{id}/members` | Lists explicit Project members; requires `project_member/read` |
| `PUT /projects/{id}/members/{user_id}` | Adds a member or changes their role; requires `project_member/create` for a new entry or `project_member/update` for an existing entry |
| `DELETE /projects/{id}/members/{user_id}` | Removes a member and reassigns their Tasks and Schedules to the Project owner atomically; requires `project_member/delete` |
| `GET /tasks` | Lists only Tasks assigned to the caller, including shared-Project Tasks |
| `GET /tasks/{id}` | Reads a Task and its attached Tags when Task read access is granted |
| `GET /tasks/{id}/assignees` | Lists the Project owner and current Project members eligible for assignment |
| `PATCH /tasks/{id}/assignees` | Changes the Task's single assignee using `{ "assignee_id": "..." }` |
| `GET /schedules` | Lists personal Schedules assigned to the caller, with each Schedule's attached Tags |
| `GET /schedules/{id}` | Reads a Schedule and its series-wide Tags when Schedule read access is granted; a live saved occurrence ID reads the same series tags |
| `GET /schedules/{id}/assignees` | Lists the Project owner and current members eligible for assignment |
| `PATCH /schedules/{id}/assignees` | Changes the Schedule's single assignee using `{ "assignee_id": "..." }` |
| `GET /schedules/{id}/revisions` | Lists Schedule snapshots, including full recurrence settings and actor |
| `GET /tags` | Lists the caller's shared Tag catalog |
| `POST /tags` | Creates a Tag in the caller's shared catalog |
| `POST /tasks/{id}/tags:add` | Attaches an owned Tag to a Task |
| `POST /schedules/{id}/tags:add` | Attaches an owned Tag to the Schedule's whole series, including when called with a live saved occurrence ID, without creating a Schedule revision |

Listing eligible assignees and changing an assignee both require the matching
`*_assignment/update` permission; resource create or update permission alone
does not grant assignment. Members can be assigned to the Project owner or any
current Project member, including a viewer. Standalone Tasks and personal
Schedules can only be assigned to their owner. Moving a Task or Schedule
between Projects requires the same owner on both Projects; removing it returns
its assignee to the Project owner. Task and Schedule ownership remain in their
respective contexts, and neither context imports the other.

## Project progress

Task progress is calculated from TodoItems only. Schedule occurrences do not
affect Task progress. Project progress is
`floor((sum of Task percentages + 100 * completed eligible Schedule occurrences) /
(Task count + eligible Schedule occurrence count))`; an empty Project has
progress `0`. Each recurring occurrence counts separately. The Schedule
context owns the occurrence eligibility rules and returns primitive counts to
the Project context for aggregation.

## Revisions and conditional writes

Projects, Tasks, and Schedules start at revision `1`. Every persisted change to
one of these entities appends a complete entity snapshot to its revision
history, including the actor and timestamps. Schedule snapshots include all
recurrence settings. Tag assignment changes do not update Schedule revisions.
History routes are `GET /api/projects/{id}/revisions`,
`GET /api/tasks/{id}/revisions`, and `GET /api/schedules/{id}/revisions`. They
use `page_size` and `page_token` and return `next_page_token`, ordered by
revision descending. History includes `created_at`, latest entity fields,
`changed_by`, and `changed_at`. TodoItem graphs are not copied into Task
history; Schedule history is owned by the Schedule context.

Single-resource reads for Projects and Tasks include a quoted revision `ETag`,
such as `ETag: "3"`. Mutations that update or delete a Project or Task, change
Task status or assignee, or move a Task into or out of a Project require that
ETag in `If-Match`. The server compares the supplied revision atomically with
the database write. A missing header returns `428`; malformed or non-positive
values return `400`; a stale revision returns `409`. Successful updates return
the new ETag; deletes return `204`. TodoItem mutations lock and recheck the live
Task and current permission in their transaction. Schedule mutations are
Schedule-owned and do not depend on Task revisions.

## Deletion and recurrence skips

Deleting a Project soft-deletes only the Project row. Its Tasks, TodoItems,
and Schedules retain their records and revisions. A trashed Project and its
children are hidden from ordinary Project and child reads and writes until the
Project is restored; restore changes only the parent. Project trash listing
requires `project/delete`. Revision history is retained and keeps the existing
`deleted_history/read` policy. Deleting a Task soft-deletes its TodoItems only;
Schedules are independent resources.

Skipping a recurrence occurrence sets `skipped_at`; it does not set
`deleted_at`. Editors may skip and restore occurrences. Resource deletion is
separate and irreversible, and skip restoration cannot resurrect a deleted
occurrence or a child of a deleted Task or Project.

## Project status and automatic lifecycle

Projects and Tasks share the five status values `open`, `in_progress`,
`pending`, `waiting_on_others`, and `done`. Status catalogs come from the shared
`status_master` table and the shared application status catalog. Migration
`000006_project_status` renames the original Task-only `task_status_master`
without changing its rows, then adds Project status and revision history against
that same master. A Project is automatically marked done when a child mutation
changes its eligible work and leaves at least one eligible item with every
direct Task done and every eligible Schedule occurrence complete. A done
Project reopens only when a child mutation adds unfinished eligible work;
unrelated edits and count-neutral changes do not reopen it. Explicit status
changes persist the requested status. A manually reopened Project remains open
even when its progress is 100; a Project whose status is done displays progress
100.

A done Project suppresses newly generated virtual recurring Schedule work and
direct operations that would materialize an unsaved occurrence. Saved Task and
Schedule children remain readable and editable, and saved skipped occurrences
can be restored. A trashed Project hides all child operations, independently of
the done-state recurrence rule.

The Project schema uses the versioned migrations under `db/migrations`; apply
all pending migrations when upgrading an existing database. The Task and
Project ER diagrams both show their foreign keys to the shared `status_master`.
