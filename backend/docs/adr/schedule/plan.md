# Independent Project, Task, Schedule, and Tag contexts

## Goal

Promote TaskSchedule to Schedule, independent of Task. Tasks contain TodoItems; Schedules own fixed-time work. Both can belong to Projects and feed daily execution plans.

Separate Project and the shared Tag catalog from the Task application context. Project owns grouping, sharing, history, and combined progress; Task owns Task/TodoItem behavior.

## Confirmed decisions

- Development schema may be rewritten; no legacy data migration or API compatibility layer is required.
- Schedule has its own owner and optional Project, following existing Task ownership and Project permission rules.
- Schedule has one assignee. Personal lists select schedules assigned to the actor; Project lists expose schedules readable within that Project.
- Task progress and automatic completion depend only on TodoItems.
- Project progress includes each Task's progress and each eligible Schedule occurrence's completion with equal weight:
  `floor((sum(task_progress) + 100 * completed_schedule_occurrences) / (task_count + schedule_occurrence_count))`.
- Empty Projects have progress zero. Preserve existing occurrence eligibility: nondeleted, nonskipped saved occurrences plus otherwise-unsaved occurrences due today in their recurrence timezone. Future virtual occurrences do not contribute.
- Project deletion applies to its Schedules as it does to Tasks. Task deletion/completion/reassignment has no effect on independent Schedules.
- Schedule has revision, changed-by attribution, and immutable change history like Task. History includes complete recurrence settings, including weekdays; weekday-only changes create a revision.
- Task and Schedule share one tag catalog, with separate assignment tables. Tag attachment/removal is excluded from revision history and must not increment Schedule revision.
- Schedule tags are series-wide: add/remove through the root or any live saved occurrence ID changes the same tag set. GET for a saved occurrence and Schedule lists include that shared tag set; list projections resolve virtual occurrences through their live backing Schedule row. A live saved override remains usable for tag operations if the series root is deleted.
- Follow-up naming cleanup: use `personal_access_token` consistently across auth responses, browser storage, generated contracts, tests, and documentation. Preserve authentication behavior.
- Remove the redundant `permission_` prefix from `permissions`' `permission_action` and `permission_effect` names, using `action` and `effect` consistently through SQL and affected contracts.

## Preserve existing behavior

- Keep Schedule CRUD, completion/reopening, rescheduling, frequency changes, skips/restores, virtual occurrences, override identity, pagination, and DST handling.
- Keep Project role allow/deny behavior and client-safe errors. An assignee does not independently grant access.
- Mirror Task defaults: personal owner and assignee are the actor; Project Schedule owner and initial assignee are the Project owner.
- Project association and assignment apply to the Schedule series, including saved overrides. This retains the existing whole-series ownership behavior previously inherited from Task. Content/date changes keep existing current/future occurrence scopes.
- Preserve TodoList associations. A daily plan generation feature is outside this extraction's implementation scope.

## Architecture

- Add `internal/application/schedule/{domain,dao,usecase,repository}` with its own UOW, repository ports, and usecase group.
- Extract the shared tag catalog into `internal/application/tag`; Task and Schedule retain their own assignment operations without importing each other or the Tag context.
- Extract Project domain, DAO, repositories, usecases, membership/sharing, revisions, and tests into `internal/application/project`. Give Project its own repository/UOW/usecase group.
- Project, Task, Schedule, and Tag do not import one another. Application root wires narrow importing-context ports to adapters. Task/Schedule keep primitive Project identifiers and their own create/association/assignment commands.
- Move common pure calendar functions into `application/shared/calendar` and recurrence cadence/frequency policy into `application/shared/recurrence`. Keep resource-specific projections, storage, and commands in their own contexts.
- Remove Schedule from Task details, progress sources, repositories, UOW, and mutation wrappers.
- Project owns primitive reader ports for Task progress and Schedule completed/total occurrence counts. Task owns per-Task progress; Schedule owns occurrence eligibility/counting. Application-root adapters map each result into Project's ports; Project computes the weighted aggregate.
- Project GET, list, and update responses combine the two progress sources. Progress stays derived; Schedule mutations do not mutate Task state.
- Preserve Project deletion, sharing, ownership, and revision behavior across the extraction, including effects on associated Task/Schedule series. Do not introduce partial multi-resource writes while separating repository/UOW ownership.
- Project deletion and member-removal reassignment use scoped Project UOW ports; root adapters bind child-context operations to the same transaction. Preserve actor attribution and child revision history.

## Database and ER

- Split migrations by responsibility: `000002_project.up/down.sql`, `000003_task.up/down.sql`, `000004_schedule.up/down.sql`, and `000005_todolist.up/down.sql`. Keep user/auth prerequisites in `000001`.
- Replace `task_schedules` with `schedules`, adding `user_id`, nullable `project_id`, and `assignee_id`; remove `task_id`.
- Enforce Project/owner consistency and recurrence series ownership. Use the existing Task rules for Project attachment and eligible assignees.
- Rename frequencies and TodoList links to `schedule_frequencies` and `todo_list_schedules`.
- Introduce direct Schedule permission checks using Project permissions. Rename permission resource/capability names from `task_schedule` to `schedule`; support assignment permissions consistently with Task.
- Rewrite Schedule CRUD/recurrence SQL and remove Schedule contributions from Task SQL. Project soft deletion handles Schedules directly.
- Keep shared frequency data and the common `tags` catalog in the Task prerequisite migration; Project prerequisites precede Task/Schedule and TodoList joins follow both. Ensure reverse migration order succeeds.
- Add `schedule_revisions` with atomic snapshots of scalar and recurrence settings. Initial history must contain complete weekdays; preserve actor attribution and immutable committed history.
- Rename the shared tag catalog to `tags`, retain `task_tag_assignments`, and add `schedule_tag_assignments` with matching target-owner validation and update permissions.
- Update seed data and split ER diagrams into `db/er/project.mmd`, `task.mmd`, `schedule.mmd`, and `todolist.mmd`, using reference-only entities for external tables.
- Regenerate sqlc from SQL source; never edit generated Go manually.

## API and documentation

- Replace Task-nested Schedule routes with independent `/schedules` routes.
- Add Schedule revision-list and tag assignment APIs. Expose the shared tag catalog through `/tags` and retain Task tag assignment routes.
- Mirror existing Task API conventions for Project list/create/association and assignee management. Preserve occurrence command semantics and list field-mask/cursor behavior.
- Use Schedule DTOs and error mappings; remove obsolete TaskSchedule paths/types and embedded schedules in Task responses.
- Regenerate Swagger after handler changes. Refresh any affected generated frontend contracts only through Orval.
- Update root/backend AGENTS, current recurrence/progress/sharing documentation, and seed/API examples to reflect the new model. Historical notes need not be rewritten as if they described the new design.

## Delegation and sequence

Main session owns clarification, this plan, coordination, and review. Research and implementation use `gpt-6-luna` with High reasoning. No agent changes another agent's assigned files without an explicit handoff.

1. **Database owner (`schedule_schema_api_audit`)**: all migrations, SQL query sources, generated sqlc, ER diagrams, seed SQL. Publish query contracts and schema fields to the other agents first. After shared extraction is complete, own removal of old Schedule code from Task, Task-only progress, extraction of the new Project context (all layers and tests), Project progress ports/aggregation, and affected Task tests. TaskTag Go files already handed to the integration owner remain excluded.
2. **Foundation/integration owner (`schedule_integration_audit`)**: first extract shared calendar/recurrence policy and adapt existing callers. Publish shared APIs. Then own shared capabilities/auth permission types, application-root Store/UOW/usecase wiring and progress adapter, REST handlers/routes/tests, generated Swagger, and current documentation updates.
   After explicit handoff, also own TaskTag Go extraction into the neutral Tag context and common catalog API.
   Own the requested personal-access-token naming cleanup across auth, frontend, and documentation; coordinate any SQL changes with the database owner and regenerate Swagger/Orval normally.
3. **Schedule owner (`schedule_domain_audit`)**: new Schedule domain/DAO/usecases/repositories and their tests. Agree on repository/query and API/usecase contracts with the other owners before wiring. Preserve Schedule behavior while removing Task dependence.

Dependency gates:

- Database owner must not start Task Go cleanup until foundation owner hands off existing Task files.
- Schedule repository generation/build depends on finalized SQL/sqlc names.
- Integration owner must use the Schedule owner's published constructors and group signatures.
- Only the database owner runs sqlc generation; only the integration owner runs Swagger generation.
- Final integration fixes remain with the relevant file owner; no overlapping bulk replacements.

## Validation and completion

- Unit tests: owner/Project/assignee validation, recurrence and DST invariants, Schedule commands, Task-only progress, and mixed/empty/Schedules-only Project progress.
- Integration tests against a disposable PostgreSQL database: independent personal Schedule CRUD, Project roles and explicit denies, assignment eligibility, series/override commands, task-deletion independence, Project deletion behavior, and progress calculations.
- Verify complete initial and updated recurrence history, weekday-only revision changes, actor attribution, and tag add/remove without revision changes; verify shared tags and target-owner permission checks.
- Verify fresh migrations, Schedule migration down/up, full rollback/reapply, and demo seed on disposable data only. Do not reset the user's configured development database.
- Preserve/rework existing Schedule coverage when moving it; do not drop coverage merely to make tests pass.
- Run Go formatting, backend tests (including the DB integration suites), and backend build. Regenerate sqlc and Swagger and check source/generated consistency.
- Check forbidden cross-context imports and remaining active TaskSchedule/task_id references; review ER and current documentation for agreement with the final schema/API.
- Verify Project and Tag implementation no longer resides in Task, while Task/Schedule association operations remain local. Re-run Project membership, ownership, deletion, revision, and progress tests against the extracted context.
- Retain concurrency regressions for Project deletion versus child creation/association and member removal versus reassignment. A deleted Project must never retain a live child created concurrently; acquire compatible Project-scoped locking within the child UOW.
- Verify auth response/storage/client naming agrees on `personal_access_token`, inspect remaining legacy-name matches, and run relevant backend/frontend auth checks.

## Validation status

- Five-step migration apply/rollback/reapply and seed smoke passed on a disposable PostgreSQL database. sqlc regeneration is stable.
- Full backend test suite and build passed with the disposable DB configured; JSON output confirmed no individual tests were skipped. `git diff --check` passed.
- Project deletion rollback/actor-history, shared-tag REST flow, and Project deletion/member-removal concurrency regressions passed. The TodoItem insertion/deletion snapshot race is covered by a deterministic regression.
- Swagger and Orval were regenerated. Frontend authentication unit tests (12), production build, and isolated login/signup E2E tests (5) passed.
- Series-wide Schedule tag behavior is confirmed: root and live saved occurrence IDs share one assignment set; Schedule GET/List responses expose it for root, saved override, and virtual occurrences. A live override remains readable after root deletion.
