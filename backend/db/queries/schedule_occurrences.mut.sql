-- name: AdvanceScheduleRevisionBackfillCheckpoint :exec
INSERT INTO schedule_backfill_checkpoints(revision_id, through_date)
VALUES (sqlc.arg(revision_id)::text, sqlc.arg(through_date)::date)
ON CONFLICT (revision_id) DO UPDATE
SET through_date = GREATEST(schedule_backfill_checkpoints.through_date, EXCLUDED.through_date), updated_at = now();

-- name: BackfillScheduleOccurrence :execrows
INSERT INTO schedules (
    id, blueprint_id, source_revision_id, user_id, occurrence_date, start_at, end_at,
    all_day, title, description, location, project_id, assignee_id, timezone, changed_by
)
SELECT sqlc.arg(id)::text, r.blueprint_id, r.id, b.user_id, sqlc.arg(occurrence_date)::date,
       sqlc.arg(start_at)::timestamptz, sqlc.arg(end_at)::timestamptz, r.all_day,
       r.title, r.description, r.location, r.project_id, r.assignee_id, r.timezone, sqlc.arg(actor_id)::text
FROM schedule_blueprint_revisions r
JOIN schedule_blueprints b ON b.id = r.blueprint_id
WHERE r.id = sqlc.arg(source_revision_id)::text AND b.deleted_at IS NULL
  AND r.frequency <> 'once'
  AND (r.end_date IS NULL OR r.end_date >= sqlc.arg(occurrence_date)::date)
  AND r.effective_from <= sqlc.arg(occurrence_date)::date
  AND (r.cutoff_after IS NULL OR sqlc.arg(occurrence_date)::date <= r.cutoff_after)
  AND r.superseded_at IS NULL
  AND (r.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = r.project_id AND p.deleted_at IS NULL))
  AND NOT EXISTS (
      SELECT 1 FROM schedule_blueprint_revisions later
      WHERE later.blueprint_id = r.blueprint_id AND later.superseded_at IS NULL
        AND later.effective_from <= sqlc.arg(occurrence_date)::date
        AND later.effective_from > r.effective_from
  )
  AND NOT EXISTS (
      SELECT 1 FROM schedule_project_pause_intervals pauses
      WHERE pauses.blueprint_id = r.blueprint_id AND pauses.project_id = r.project_id
        AND pauses.paused_at <= sqlc.arg(start_at)::timestamptz
        AND (pauses.resumed_at IS NULL OR sqlc.arg(start_at)::timestamptz < pauses.resumed_at)
  )
ON CONFLICT (blueprint_id, occurrence_date) DO NOTHING;

-- name: CreateScheduleOccurrence :one
INSERT INTO schedules (
    id, blueprint_id, source_revision_id, user_id, occurrence_date, start_at, end_at,
    all_day, title, description, location, project_id, assignee_id, timezone, changed_by
)
SELECT sqlc.arg(id)::text, r.blueprint_id, r.id, r.user_id, sqlc.arg(occurrence_date)::date,
       sqlc.arg(start_at)::timestamptz, sqlc.arg(end_at)::timestamptz, r.all_day,
       r.title, r.description, r.location, r.project_id, r.assignee_id, r.timezone, sqlc.arg(actor_id)::text
FROM schedule_blueprint_revisions r
WHERE r.id = sqlc.arg(source_revision_id)::text
ON CONFLICT (blueprint_id, occurrence_date) DO NOTHING
RETURNING *;

-- name: DeleteProjectBlueprintOccurrencesByActor :execrows
UPDATE schedules s
SET deleted_at = now(), changed_by = sqlc.arg(actor_id)::text, updated_at = now()
WHERE s.project_id = sqlc.arg(project_id)::text AND s.deleted_at IS NULL;

-- name: GetScheduleRevisionBackfillCheckpoint :one
SELECT through_date FROM schedule_backfill_checkpoints
WHERE revision_id = sqlc.arg(revision_id)::text;

-- name: ListScheduleOccurrenceProjectIDsForFollowing :many
SELECT DISTINCT occurrence.project_id
FROM schedules occurrence
WHERE occurrence.blueprint_id = sqlc.arg(blueprint_id)::text
  AND occurrence.occurrence_date >= sqlc.arg(occurrence_date)::date
  AND occurrence.deleted_at IS NULL
  AND occurrence.project_id IS NOT NULL
ORDER BY occurrence.project_id;

-- name: LockScheduleBlueprintForBackfill :one
SELECT id FROM schedule_blueprints
WHERE id = sqlc.arg(blueprint_id)::text AND deleted_at IS NULL
FOR UPDATE;

-- name: LockScheduleBlueprintForRevisionMutation :one
SELECT blueprint.id
FROM schedule_blueprints blueprint
WHERE blueprint.id = sqlc.arg(blueprint_id)::text
  AND blueprint.deleted_at IS NULL
  AND schedule_blueprint_has_permission_at(blueprint.id, sqlc.arg(occurrence_date)::date,
      sqlc.arg(actor_id)::text, sqlc.arg(resource_id)::text, sqlc.arg(action)::action)
FOR UPDATE;

-- name: LockScheduleOccurrenceForMutation :one
WITH locked AS (
    SELECT b.id
    FROM schedule_blueprints b
    WHERE b.id = sqlc.arg(blueprint_id)::text AND b.deleted_at IS NULL
      AND (
        EXISTS (SELECT 1 FROM schedules s WHERE s.blueprint_id = b.id AND s.occurrence_date = sqlc.arg(occurrence_date)::date
            AND schedule_occurrence_has_permission(s.id, sqlc.arg(actor_id)::text,
                sqlc.arg(resource_id)::text, sqlc.arg(action)::action))
        OR (NOT EXISTS (SELECT 1 FROM schedules s WHERE s.blueprint_id = b.id AND s.occurrence_date = sqlc.arg(occurrence_date)::date)
            AND schedule_blueprint_has_permission_at(b.id, sqlc.arg(occurrence_date)::date, sqlc.arg(actor_id)::text,
                sqlc.arg(resource_id)::text, sqlc.arg(action)::action))
      )
      FOR UPDATE
), saved AS (
    SELECT s.id
    FROM schedules s JOIN locked b ON b.id = s.blueprint_id
    WHERE s.occurrence_date = sqlc.arg(occurrence_date)::date
      AND schedule_occurrence_has_permission(s.id, sqlc.arg(actor_id)::text,
          sqlc.arg(resource_id)::text, sqlc.arg(action)::action)
    FOR UPDATE OF s
)
SELECT id FROM locked WHERE (SELECT count(*) FROM saved) >= 0;

-- name: MaterializeScheduleOccurrenceByActor :one
INSERT INTO schedules (
    id, blueprint_id, source_revision_id, user_id, occurrence_date, start_at, end_at,
    all_day, title, description, location, project_id, assignee_id, timezone, completed,
    skipped_at, deleted_at, is_override, changed_by
)
SELECT sqlc.arg(id)::text, r.blueprint_id, r.id, r.user_id, sqlc.arg(occurrence_date)::date,
       sqlc.arg(start_at)::timestamptz, sqlc.arg(end_at)::timestamptz, sqlc.arg(all_day)::boolean,
       sqlc.arg(title)::text, sqlc.narg(description)::text, sqlc.narg(location)::text, sqlc.narg(project_id)::text,
       sqlc.arg(assignee_id)::text, sqlc.arg(timezone)::text, sqlc.arg(completed)::boolean,
       sqlc.narg(skipped_at)::timestamptz, sqlc.narg(deleted_at)::timestamptz,
       sqlc.arg(is_override)::boolean, sqlc.arg(actor_id)::text
FROM schedule_blueprint_revisions r
WHERE r.id = sqlc.arg(source_revision_id)::text
  AND r.blueprint_id = sqlc.arg(blueprint_id)::text
  AND schedule_blueprint_has_permission_at(r.blueprint_id, sqlc.arg(occurrence_date)::date,
      sqlc.arg(actor_id)::text, sqlc.arg(resource_id)::text, sqlc.arg(action)::action)
ON CONFLICT (blueprint_id, occurrence_date) DO NOTHING
RETURNING *;

-- name: ReassignProjectMemberBlueprintOccurrencesByActor :execrows
UPDATE schedules s
SET assignee_id = p.user_id, changed_by = sqlc.arg(actor_id)::text, updated_at = now()
FROM projects p
WHERE s.project_id = p.id AND p.id = sqlc.arg(project_id)::text
  AND s.assignee_id = sqlc.arg(member_id)::text;

-- name: UpdatePersistedScheduleOccurrenceByActor :one
UPDATE schedules s
SET source_revision_id = sqlc.arg(source_revision_id)::text,
    start_at = sqlc.arg(start_at)::timestamptz, end_at = sqlc.arg(end_at)::timestamptz,
    all_day = sqlc.arg(all_day)::boolean,
    title = sqlc.arg(title)::text, description = sqlc.narg(description)::text,
    location = sqlc.narg(location)::text, project_id = sqlc.narg(project_id)::text,
    assignee_id = sqlc.arg(assignee_id)::text, timezone = sqlc.arg(timezone)::text,
    completed = sqlc.arg(completed)::boolean, skipped_at = sqlc.narg(skipped_at)::timestamptz,
    deleted_at = sqlc.narg(deleted_at)::timestamptz, is_override = sqlc.arg(is_override)::boolean,
    changed_by = sqlc.arg(actor_id)::text
WHERE s.blueprint_id = sqlc.arg(blueprint_id)::text
  AND s.occurrence_date = sqlc.arg(occurrence_date)::date
  AND s.revision = sqlc.arg(expected_revision)::integer
  AND schedule_occurrence_has_permission(s.id, sqlc.arg(actor_id)::text,
      sqlc.arg(resource_id)::text, sqlc.arg(action)::action)
RETURNING s.*;

-- name: UpsertScheduleOccurrenceByActor :one
INSERT INTO schedules (
    id, blueprint_id, source_revision_id, user_id, occurrence_date, start_at, end_at,
    all_day, title, description, location, project_id, assignee_id, timezone, completed,
    skipped_at, deleted_at, is_override, changed_by
)
SELECT sqlc.arg(id)::text, sqlc.arg(blueprint_id)::text, sqlc.arg(source_revision_id)::text,
       sqlc.arg(user_id)::text, sqlc.arg(occurrence_date)::date, sqlc.arg(start_at)::timestamptz,
       sqlc.arg(end_at)::timestamptz, sqlc.arg(all_day)::boolean, sqlc.arg(title)::text, sqlc.narg(description)::text,
       sqlc.narg(location)::text, sqlc.narg(project_id)::text, sqlc.arg(assignee_id)::text,
       sqlc.arg(timezone)::text, sqlc.arg(completed)::boolean, sqlc.narg(skipped_at)::timestamptz,
       sqlc.narg(deleted_at)::timestamptz, sqlc.arg(is_override)::boolean, sqlc.arg(actor_id)::text
WHERE schedule_blueprint_has_permission_at(sqlc.arg(blueprint_id)::text, sqlc.arg(occurrence_date)::date, sqlc.arg(actor_id)::text, sqlc.arg(resource_id)::text, sqlc.arg(action)::action)
ON CONFLICT (blueprint_id, occurrence_date) DO UPDATE
SET source_revision_id = EXCLUDED.source_revision_id,
    start_at = EXCLUDED.start_at, end_at = EXCLUDED.end_at,
    all_day = EXCLUDED.all_day,
    title = EXCLUDED.title, description = EXCLUDED.description, location = EXCLUDED.location,
    project_id = EXCLUDED.project_id, assignee_id = EXCLUDED.assignee_id, timezone = EXCLUDED.timezone,
    completed = EXCLUDED.completed, skipped_at = EXCLUDED.skipped_at, deleted_at = EXCLUDED.deleted_at,
    is_override = EXCLUDED.is_override, changed_by = EXCLUDED.changed_by
WHERE schedules.revision = sqlc.arg(expected_revision)::integer
  AND schedule_has_permission(schedules.id, sqlc.arg(actor_id)::text, sqlc.arg(resource_id)::text, sqlc.arg(action)::action)
RETURNING *;
