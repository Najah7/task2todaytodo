-- name: UpsertScheduleOverrideByUserID :one
INSERT INTO schedules (
    id, user_id, project_id, assignee_id, title, description, location, start_at, end_at,
    series_id, occurrence_date, timezone, is_exception, completed,
    repeat_state, frequency_anchor_date, interval_weeks, deleted_at, skipped_at, changed_by
)
SELECT sqlc.arg(id)::text, root.user_id, root.project_id, root.assignee_id,
       sqlc.arg(title)::text, sqlc.narg(description)::text, sqlc.narg(location)::text,
       sqlc.arg(start_at)::timestamptz, sqlc.arg(end_at)::timestamptz,
       root.id, sqlc.arg(occurrence_date)::date, sqlc.arg(timezone)::text, true,
       sqlc.arg(completed)::boolean, NULL, NULL, 0,
       CASE WHEN sqlc.arg(deleted)::boolean THEN now() ELSE NULL END, NULL, sqlc.arg(user_id)::text
FROM schedules root
WHERE root.id = sqlc.arg(series_id)::text
  AND root.id = root.series_id
  AND root.deleted_at IS NULL
  AND schedule_has_permission(root.id, sqlc.arg(user_id)::text, 'schedule', 'update')
ON CONFLICT (series_id, occurrence_date) WHERE id <> series_id DO UPDATE SET
    title = EXCLUDED.title,
    description = EXCLUDED.description,
    location = EXCLUDED.location,
    start_at = EXCLUDED.start_at,
    end_at = EXCLUDED.end_at,
    timezone = EXCLUDED.timezone,
    is_exception = true,
    completed = EXCLUDED.completed,
    deleted_at = EXCLUDED.deleted_at,
    changed_by = EXCLUDED.changed_by,
    updated_at = now()
WHERE schedules.deleted_at IS NULL
RETURNING id;

-- name: SkipScheduleOccurrenceByUserID :one
INSERT INTO schedules (
    id, user_id, project_id, assignee_id, title, description, location, start_at, end_at,
    series_id, occurrence_date, timezone, is_exception, completed,
    repeat_state, frequency_anchor_date, interval_weeks, deleted_at, skipped_at, changed_by
)
SELECT sqlc.arg(id)::text, root.user_id, root.project_id, root.assignee_id,
       root.title, root.description, root.location, root.start_at, root.end_at,
       root.id, sqlc.arg(occurrence_date)::date, root.timezone, false,
       false, NULL, NULL, 0, NULL, now(), sqlc.arg(user_id)::text
FROM schedules root
WHERE root.id = sqlc.arg(series_id)::text
  AND root.id = root.series_id
  AND root.deleted_at IS NULL
  AND schedule_has_permission(root.id, sqlc.arg(user_id)::text, 'occurrence', 'update')
ON CONFLICT (series_id, occurrence_date) WHERE id <> series_id
DO UPDATE SET skipped_at = COALESCE(schedules.skipped_at, now()), changed_by = sqlc.arg(user_id)::text, updated_at = now()
WHERE schedules.deleted_at IS NULL
RETURNING id;

-- name: RestoreScheduleOccurrenceByUserID :exec
UPDATE schedules child
SET skipped_at = NULL, changed_by = sqlc.arg(user_id)::text, updated_at = now()
FROM schedules root
WHERE child.series_id = root.id
  AND child.series_id = sqlc.arg(series_id)::text
  AND child.occurrence_date = sqlc.arg(occurrence_date)::date
  AND child.id <> child.series_id
  AND child.is_exception
  AND child.deleted_at IS NULL
  AND child.skipped_at IS NOT NULL
  AND root.deleted_at IS NULL
  AND root.id = root.series_id
  AND schedule_has_permission(root.id, sqlc.arg(user_id)::text, 'occurrence', 'update');

-- name: DeleteSkippedScheduleOccurrenceByUserID :exec
UPDATE schedules child
SET deleted_at = now(), changed_by = sqlc.arg(user_id)::text, updated_at = now()
FROM schedules root
WHERE child.series_id = root.id
  AND child.series_id = sqlc.arg(series_id)::text
  AND child.occurrence_date = sqlc.arg(occurrence_date)::date
  AND child.id <> child.series_id
  AND NOT child.is_exception
  AND child.deleted_at IS NULL
  AND child.skipped_at IS NOT NULL
  AND root.deleted_at IS NULL
  AND root.id = root.series_id
  AND schedule_has_permission(root.id, sqlc.arg(user_id)::text, 'occurrence', 'update');
