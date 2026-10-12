-- name: AdvanceScheduleRevisionBackfillCheckpoint :exec
INSERT INTO schedule_backfill_checkpoints(revision_id, through_date)
VALUES (sqlc.arg(revision_id)::text, sqlc.arg(through_date)::date)
ON CONFLICT (revision_id) DO UPDATE
SET through_date = GREATEST(schedule_backfill_checkpoints.through_date, EXCLUDED.through_date), updated_at = now();

-- name: BackfillScheduleOccurrence :execrows
INSERT INTO schedules (
    id, rule_id, source_revision_id, user_id, occurrence_date, start_at, end_at,
    all_day, title, description, location, project_id, assignee_id, timezone, changed_by
)
SELECT sqlc.arg(id)::text, r.rule_id, r.id, b.user_id, sqlc.arg(occurrence_date)::date,
       sqlc.arg(start_at)::timestamptz, sqlc.arg(end_at)::timestamptz, r.all_day,
       r.title, r.description, r.location, r.project_id, r.assignee_id, r.timezone, sqlc.arg(actor_id)::text
FROM schedule_rule_revisions r
JOIN schedule_rules b ON b.id = r.rule_id
WHERE r.id = sqlc.arg(source_revision_id)::text AND b.deleted_at IS NULL
  AND r.frequency <> 'once'
  AND (r.end_date IS NULL OR r.end_date >= sqlc.arg(occurrence_date)::date)
  AND r.effective_from <= sqlc.arg(occurrence_date)::date
  AND (r.cutoff_after IS NULL OR sqlc.arg(occurrence_date)::date <= r.cutoff_after)
  AND r.superseded_at IS NULL
  AND (r.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = r.project_id AND p.deleted_at IS NULL))
  AND NOT EXISTS (
      SELECT 1 FROM schedule_rule_revisions later
      WHERE later.rule_id = r.rule_id AND later.superseded_at IS NULL
        AND later.effective_from <= sqlc.arg(occurrence_date)::date
        AND later.effective_from > r.effective_from
  )
  AND NOT EXISTS (
      SELECT 1 FROM schedule_project_pause_intervals pauses
      WHERE pauses.rule_id = r.rule_id AND pauses.project_id = r.project_id
        AND pauses.paused_at <= sqlc.arg(start_at)::timestamptz
        AND (pauses.resumed_at IS NULL OR sqlc.arg(start_at)::timestamptz < pauses.resumed_at)
  )
  AND NOT EXISTS (
      SELECT 1 FROM schedule_exceptions exception_row
      WHERE exception_row.rule_id = r.rule_id AND exception_row.occurrence_date = sqlc.arg(occurrence_date)::date
  )
ON CONFLICT (rule_id, occurrence_date) DO NOTHING;

-- name: CreateScheduleOccurrence :one
INSERT INTO schedules (
    id, rule_id, source_revision_id, user_id, occurrence_date, start_at, end_at,
    all_day, title, description, location, project_id, assignee_id, timezone, changed_by
)
SELECT sqlc.arg(id)::text, r.rule_id, r.id, r.user_id, sqlc.arg(occurrence_date)::date,
       sqlc.arg(start_at)::timestamptz, sqlc.arg(end_at)::timestamptz, r.all_day,
       r.title, r.description, r.location, r.project_id, r.assignee_id, r.timezone, sqlc.arg(actor_id)::text
FROM schedule_rule_revisions r
WHERE r.id = sqlc.arg(source_revision_id)::text
ON CONFLICT (rule_id, occurrence_date) DO NOTHING
RETURNING *;

-- name: DeleteProjectRuleOccurrencesByActor :execrows
WITH exception_changes AS (
    UPDATE schedule_exceptions
    SET deleted_at = now(), changed_by = sqlc.arg(actor_id)::text, updated_at = now()
    WHERE project_id = sqlc.arg(project_id)::text AND deleted_at IS NULL
    RETURNING rule_id, occurrence_date
)
UPDATE schedules s
SET deleted_at = now(), changed_by = sqlc.arg(actor_id)::text, updated_at = now()
WHERE s.project_id = sqlc.arg(project_id)::text AND s.deleted_at IS NULL;

-- name: GetScheduleRevisionBackfillCheckpoint :one
SELECT through_date FROM schedule_backfill_checkpoints
WHERE revision_id = sqlc.arg(revision_id)::text;

-- name: ListScheduleOccurrenceProjectIDsForFollowing :many
SELECT DISTINCT occurrence.project_id
FROM schedules occurrence
WHERE occurrence.rule_id = sqlc.arg(rule_id)::text
  AND occurrence.occurrence_date >= sqlc.arg(occurrence_date)::date
  AND occurrence.deleted_at IS NULL
  AND occurrence.project_id IS NOT NULL
ORDER BY occurrence.project_id;

-- name: LockScheduleRuleForBackfill :one
SELECT id FROM schedule_rules
WHERE id = sqlc.arg(rule_id)::text AND deleted_at IS NULL
FOR UPDATE;

-- name: LockScheduleRuleForRevisionMutation :one
SELECT rule.id
FROM schedule_rules rule
WHERE rule.id = sqlc.arg(rule_id)::text
  AND rule.deleted_at IS NULL
  AND schedule_rule_has_permission_at(rule.id, sqlc.arg(occurrence_date)::date,
      sqlc.arg(actor_id)::text, sqlc.arg(resource_id)::text, sqlc.arg(action)::action)
FOR UPDATE;

-- name: LockScheduleOccurrenceForMutation :one
WITH locked AS (
    SELECT b.id
    FROM schedule_rules b
    WHERE b.id = sqlc.arg(rule_id)::text AND b.deleted_at IS NULL
      AND (
        EXISTS (SELECT 1 FROM schedules s WHERE s.rule_id = b.id AND s.occurrence_date = sqlc.arg(occurrence_date)::date
            AND schedule_occurrence_has_permission(s.id, sqlc.arg(actor_id)::text,
                sqlc.arg(resource_id)::text, sqlc.arg(action)::action))
        OR (NOT EXISTS (SELECT 1 FROM schedules s WHERE s.rule_id = b.id AND s.occurrence_date = sqlc.arg(occurrence_date)::date)
            AND schedule_rule_has_permission_at(b.id, sqlc.arg(occurrence_date)::date, sqlc.arg(actor_id)::text,
                sqlc.arg(resource_id)::text, sqlc.arg(action)::action))
      )
      FOR UPDATE
), saved AS (
    SELECT s.id
    FROM schedules s JOIN locked b ON b.id = s.rule_id
    WHERE s.occurrence_date = sqlc.arg(occurrence_date)::date
      AND schedule_occurrence_has_permission(s.id, sqlc.arg(actor_id)::text,
          sqlc.arg(resource_id)::text, sqlc.arg(action)::action)
    FOR UPDATE OF s
)
SELECT id FROM locked WHERE (SELECT count(*) FROM saved) >= 0;

-- name: MaterializeScheduleOccurrenceByActor :one
WITH saved_exception AS (
    INSERT INTO schedule_exceptions (
        rule_id, occurrence_date, source_revision_id, user_id, start_at, end_at,
        all_day, title, description, location, project_id, assignee_id, timezone,
        completed, skipped_at, deleted_at, changed_by
    )
    SELECT r.rule_id, sqlc.arg(occurrence_date)::date, r.id, r.user_id,
           sqlc.arg(start_at)::timestamptz, sqlc.arg(end_at)::timestamptz,
           sqlc.arg(all_day)::boolean, sqlc.arg(title)::text, sqlc.narg(description)::text,
           sqlc.narg(location)::text, sqlc.narg(project_id)::text, sqlc.arg(assignee_id)::text,
           sqlc.arg(timezone)::text, sqlc.arg(completed)::boolean,
           sqlc.narg(skipped_at)::timestamptz, sqlc.narg(deleted_at)::timestamptz,
           sqlc.arg(actor_id)::text
    FROM schedule_rule_revisions r
    WHERE r.id = sqlc.arg(source_revision_id)::text
      AND r.rule_id = sqlc.arg(rule_id)::text
      AND schedule_rule_has_permission_at(r.rule_id, sqlc.arg(occurrence_date)::date,
          sqlc.arg(actor_id)::text, sqlc.arg(resource_id)::text, sqlc.arg(action)::action)
    ON CONFLICT (rule_id, occurrence_date) DO NOTHING
    RETURNING *
)
INSERT INTO schedules (
    id, rule_id, source_revision_id, user_id, occurrence_date, start_at, end_at,
    all_day, title, description, location, project_id, assignee_id, timezone,
    completed, skipped_at, deleted_at, changed_by
)
SELECT sqlc.arg(id)::text, exception_row.rule_id, exception_row.source_revision_id, exception_row.user_id,
       exception_row.occurrence_date, exception_row.start_at, exception_row.end_at, exception_row.all_day,
       exception_row.title, exception_row.description, exception_row.location, exception_row.project_id,
       exception_row.assignee_id, exception_row.timezone, exception_row.completed, exception_row.skipped_at,
       exception_row.deleted_at, exception_row.changed_by
FROM saved_exception exception_row
RETURNING *;

-- name: UpdatePersistedScheduleOccurrenceByActor :one
WITH locked AS (
    SELECT s.id
    FROM schedules s
    WHERE s.rule_id = sqlc.arg(rule_id)::text
      AND s.occurrence_date = sqlc.arg(occurrence_date)::date
      AND s.revision = sqlc.arg(expected_revision)::integer
      AND schedule_occurrence_has_permission(s.id, sqlc.arg(actor_id)::text,
          sqlc.arg(resource_id)::text, sqlc.arg(action)::action)
    FOR UPDATE
), saved_exception AS (
    INSERT INTO schedule_exceptions (
        rule_id, occurrence_date, source_revision_id, user_id, start_at, end_at,
        all_day, title, description, location, project_id, assignee_id, timezone,
        completed, skipped_at, deleted_at, changed_by
    )
    SELECT s.rule_id, s.occurrence_date, sqlc.arg(source_revision_id)::text, s.user_id,
           sqlc.arg(start_at)::timestamptz, sqlc.arg(end_at)::timestamptz,
           sqlc.arg(all_day)::boolean, sqlc.arg(title)::text, sqlc.narg(description)::text,
           sqlc.narg(location)::text, sqlc.narg(project_id)::text, sqlc.arg(assignee_id)::text,
           sqlc.arg(timezone)::text, sqlc.arg(completed)::boolean,
           sqlc.narg(skipped_at)::timestamptz, sqlc.narg(deleted_at)::timestamptz,
           sqlc.arg(actor_id)::text
    FROM schedules s JOIN locked ON locked.id = s.id
    ON CONFLICT (rule_id, occurrence_date) DO UPDATE SET
        source_revision_id = EXCLUDED.source_revision_id,
        start_at = EXCLUDED.start_at, end_at = EXCLUDED.end_at, all_day = EXCLUDED.all_day,
        title = EXCLUDED.title, description = EXCLUDED.description, location = EXCLUDED.location,
        project_id = EXCLUDED.project_id, assignee_id = EXCLUDED.assignee_id,
        timezone = EXCLUDED.timezone, completed = EXCLUDED.completed,
        skipped_at = EXCLUDED.skipped_at, deleted_at = EXCLUDED.deleted_at,
        changed_by = EXCLUDED.changed_by, updated_at = now()
    RETURNING *
)
UPDATE schedules s
SET source_revision_id = exception_row.source_revision_id,
    start_at = exception_row.start_at, end_at = exception_row.end_at, all_day = exception_row.all_day,
    title = exception_row.title, description = exception_row.description, location = exception_row.location,
    project_id = exception_row.project_id, assignee_id = exception_row.assignee_id,
    timezone = exception_row.timezone, completed = exception_row.completed,
    skipped_at = exception_row.skipped_at, deleted_at = exception_row.deleted_at,
    changed_by = exception_row.changed_by
FROM saved_exception exception_row
WHERE s.rule_id = exception_row.rule_id AND s.occurrence_date = exception_row.occurrence_date
RETURNING s.*;

-- name: ReassignProjectMemberRuleOccurrencesByActor :execrows
WITH project AS (
    SELECT id, user_id FROM projects WHERE id = sqlc.arg(project_id)::text
), exception_changes AS (
    UPDATE schedule_exceptions exception_row
    SET assignee_id = project.user_id, changed_by = sqlc.arg(actor_id)::text, updated_at = now()
    FROM project
    WHERE exception_row.project_id = project.id AND exception_row.assignee_id = sqlc.arg(member_id)::text
    RETURNING exception_row.rule_id, exception_row.occurrence_date
)
UPDATE schedules s
SET assignee_id = project.user_id, changed_by = sqlc.arg(actor_id)::text, updated_at = now()
FROM project
WHERE s.project_id = project.id AND s.assignee_id = sqlc.arg(member_id)::text;

-- name: ApplyScheduleRuleRevisionToOccurrence :execrows
UPDATE schedules s
SET source_revision_id = sqlc.arg(source_revision_id)::text,
    start_at = sqlc.arg(start_at)::timestamptz, end_at = sqlc.arg(end_at)::timestamptz,
    all_day = sqlc.arg(all_day)::boolean,
    title = sqlc.arg(title)::text, description = sqlc.narg(description)::text,
    location = sqlc.narg(location)::text, project_id = sqlc.narg(project_id)::text,
    assignee_id = sqlc.arg(assignee_id)::text, timezone = sqlc.arg(timezone)::text,
    changed_by = sqlc.arg(actor_id)::text
WHERE s.rule_id = sqlc.arg(rule_id)::text
  AND s.occurrence_date = sqlc.arg(occurrence_date)::date
  AND NOT EXISTS (
      SELECT 1 FROM schedule_exceptions exception_row
      WHERE exception_row.rule_id = s.rule_id AND exception_row.occurrence_date = s.occurrence_date
  );
