-- name: GetTaskSchedule :one
SELECT
    ts.id,
    ts.task_id,
    ts.title,
    ts.description,
    ts.location,
    COALESCE((SELECT r.interval_weeks FROM task_schedules r WHERE r.id = ts.series_id ), 0)::integer AS interval_weeks,
    (SELECT r.repeat_state FROM task_schedules r WHERE r.id = ts.series_id) AS repeat_state,
    (SELECT r.frequency_anchor_date FROM task_schedules r WHERE r.id = ts.series_id) AS frequency_anchor_date,
    ts.series_id,
    ts.occurrence_date,
    ts.timezone,
    ts.is_exception,
    ts.completed,
    (ts.deleted_at IS NOT NULL) AS deleted,
    ARRAY(
        SELECT tsf.frequency
        FROM task_schedule_frequencies AS tsf
        WHERE tsf.task_schedule_id = ts.series_id
        ORDER BY tsf.frequency
    )::text[] AS frequencies,
    ts.start_at,
    ts.end_at,
    ts.created_at,
    ts.updated_at
FROM task_schedules AS ts
WHERE ts.id = $1
  AND ts.deleted_at IS NULL;

-- name: GetTaskScheduleByTaskAndUserID :one
SELECT
    ts.id,
    ts.task_id,
    ts.title,
    ts.description,
    ts.location,
    COALESCE((SELECT r.interval_weeks FROM task_schedules r WHERE r.id = ts.series_id ), 0)::integer AS interval_weeks,
    (SELECT r.repeat_state FROM task_schedules r WHERE r.id = ts.series_id) AS repeat_state,
    (SELECT r.frequency_anchor_date FROM task_schedules r WHERE r.id = ts.series_id) AS frequency_anchor_date,
    ts.series_id,
    ts.occurrence_date,
    ts.timezone,
    ts.is_exception,
    ts.completed,
    (ts.deleted_at IS NOT NULL) AS deleted,
    ARRAY(
        SELECT tsf.frequency
        FROM task_schedule_frequencies AS tsf
        WHERE tsf.task_schedule_id = ts.series_id
        ORDER BY tsf.frequency
    )::text[] AS frequencies,
    ts.start_at,
    ts.end_at,
    ts.created_at,
    ts.updated_at
FROM task_schedules AS ts
JOIN tasks AS t ON t.id = ts.task_id
WHERE ts.id = $1
  AND ts.task_id = $2
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, $3, 'task_schedule', 'read')
  AND ts.deleted_at IS NULL;

-- name: GetTaskScheduleByTaskAndUserIDForCommand :one
SELECT
    ts.id,
    ts.task_id,
    ts.title,
    ts.description,
    ts.location,
    COALESCE((SELECT r.interval_weeks FROM task_schedules r WHERE r.id = ts.series_id), 0)::integer AS interval_weeks,
    (SELECT r.repeat_state FROM task_schedules r WHERE r.id = ts.series_id) AS repeat_state,
    (SELECT r.frequency_anchor_date FROM task_schedules r WHERE r.id = ts.series_id) AS frequency_anchor_date,
    ts.series_id,
    ts.occurrence_date,
    ts.timezone,
    ts.is_exception,
    ts.completed,
    (ts.deleted_at IS NOT NULL) AS deleted,
    ARRAY(
        SELECT tsf.frequency
        FROM task_schedule_frequencies AS tsf
        WHERE tsf.task_schedule_id = ts.series_id
        ORDER BY tsf.frequency
    )::text[] AS frequencies,
    ts.start_at,
    ts.end_at,
    ts.created_at,
    ts.updated_at
FROM task_schedules AS ts
JOIN tasks AS t ON t.id = ts.task_id
WHERE ts.id = sqlc.arg(id)::text
  AND ts.task_id = sqlc.arg(task_id)::text
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(actor_id)::text, sqlc.arg(resource_id)::text, sqlc.arg(action)::permission_action);

-- name: ListTaskSchedulesByTaskAndUserID :many
SELECT
    ts.id,
    ts.task_id,
    ts.title,
    ts.description,
    ts.location,
    COALESCE((SELECT r.interval_weeks FROM task_schedules r WHERE r.id = ts.series_id ), 0)::integer AS interval_weeks,
    (SELECT r.repeat_state FROM task_schedules r WHERE r.id = ts.series_id) AS repeat_state,
    (SELECT r.frequency_anchor_date FROM task_schedules r WHERE r.id = ts.series_id) AS frequency_anchor_date,
    ts.series_id,
    ts.occurrence_date,
    ts.timezone,
    ts.is_exception,
    ts.completed,
    (ts.deleted_at IS NOT NULL) AS deleted,
    ARRAY(
        SELECT tsf.frequency
        FROM task_schedule_frequencies AS tsf
        WHERE tsf.task_schedule_id = ts.series_id
        ORDER BY tsf.frequency
    )::text[] AS frequencies,
    ts.start_at,
    ts.end_at,
    ts.created_at,
    ts.updated_at
FROM task_schedules AS ts
JOIN tasks AS t ON t.id = ts.task_id
WHERE ts.task_id = $1
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, $2, 'task_schedule', 'read')
  AND ts.deleted_at IS NULL
ORDER BY ts.start_at ASC;

-- name: ListTaskSchedulesForOccurrenceProjectionByTaskAndUserID :many
SELECT
    ts.id,
    ts.task_id,
    ts.title,
    ts.description,
    ts.location,
    COALESCE((SELECT r.interval_weeks FROM task_schedules r WHERE r.id = ts.series_id ), 0)::integer AS interval_weeks,
    (SELECT r.repeat_state FROM task_schedules r WHERE r.id = ts.series_id) AS repeat_state,
    (SELECT r.frequency_anchor_date FROM task_schedules r WHERE r.id = ts.series_id) AS frequency_anchor_date,
    ts.series_id,
    ts.occurrence_date,
    ts.timezone,
    ts.is_exception,
    ts.completed,
    (ts.deleted_at IS NOT NULL) AS deleted,
    ARRAY(
        SELECT tsf.frequency
        FROM task_schedule_frequencies AS tsf
        WHERE tsf.task_schedule_id = ts.series_id
        ORDER BY tsf.frequency
    )::text[] AS frequencies,
    ts.start_at,
    ts.end_at,
    ts.created_at,
    ts.updated_at
FROM task_schedules AS ts
JOIN tasks AS t ON t.id = ts.task_id
WHERE ts.task_id = $1
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, $2, 'task_schedule', 'read')
ORDER BY ts.start_at ASC;

-- name: ListTaskSchedulesForOccurrenceCommandByTaskAndUserID :many
SELECT
    ts.id,
    ts.task_id,
    ts.title,
    ts.description,
    ts.location,
    COALESCE((SELECT r.interval_weeks FROM task_schedules r WHERE r.id = ts.series_id), 0)::integer AS interval_weeks,
    (SELECT r.repeat_state FROM task_schedules r WHERE r.id = ts.series_id) AS repeat_state,
    (SELECT r.frequency_anchor_date FROM task_schedules r WHERE r.id = ts.series_id) AS frequency_anchor_date,
    ts.series_id,
    ts.occurrence_date,
    ts.timezone,
    ts.is_exception,
    ts.completed,
    (ts.deleted_at IS NOT NULL) AS deleted,
    ARRAY(
        SELECT tsf.frequency
        FROM task_schedule_frequencies AS tsf
        WHERE tsf.task_schedule_id = ts.series_id
        ORDER BY tsf.frequency
    )::text[] AS frequencies,
    ts.start_at,
    ts.end_at,
    ts.created_at,
    ts.updated_at
FROM task_schedules AS ts
JOIN tasks AS t ON t.id = ts.task_id
WHERE ts.task_id = sqlc.arg(task_id)::text
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(actor_id)::text, sqlc.arg(resource_id)::text, sqlc.arg(action)::permission_action)
ORDER BY ts.start_at ASC, ts.occurrence_date ASC;

-- name: ListTaskSchedulesByTaskAndUserIDCursorPage :many
SELECT
    ts.id,
    ts.task_id,
    ts.title,
    ts.description,
    ts.location,
    COALESCE((SELECT r.interval_weeks FROM task_schedules r WHERE r.id = ts.series_id ), 0)::integer AS interval_weeks,
    (SELECT r.repeat_state FROM task_schedules r WHERE r.id = ts.series_id) AS repeat_state,
    (SELECT r.frequency_anchor_date FROM task_schedules r WHERE r.id = ts.series_id) AS frequency_anchor_date,
    ts.series_id,
    ts.occurrence_date,
    ts.timezone,
    ts.is_exception,
    ts.completed,
    (ts.deleted_at IS NOT NULL) AS deleted,
    ARRAY(
        SELECT tsf.frequency
        FROM task_schedule_frequencies AS tsf
        WHERE tsf.task_schedule_id = ts.series_id
        ORDER BY tsf.frequency
    )::text[] AS frequencies,
    ts.start_at,
    ts.end_at,
    ts.created_at,
    ts.updated_at
FROM task_schedules AS ts
JOIN tasks AS t ON t.id = ts.task_id
WHERE ts.task_id = $1
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, $2, 'task_schedule', 'read')
  AND ts.deleted_at IS NULL
  AND (sqlc.narg(cursor_at)::timestamptz IS NULL OR (ts.start_at, ts.id) > (sqlc.narg(cursor_at)::timestamptz, sqlc.narg(cursor_id)::text))
ORDER BY ts.start_at ASC, ts.id ASC
LIMIT sqlc.arg(page_limit)::integer;

-- name: ListActiveTaskScheduleSeriesByUserID :many
SELECT
    ts.id,
    ts.task_id,
    ts.title,
    ts.description,
    ts.location,
    COALESCE((SELECT r.interval_weeks FROM task_schedules r WHERE r.id = ts.series_id ), 0)::integer AS interval_weeks,
    (SELECT r.repeat_state FROM task_schedules r WHERE r.id = ts.series_id) AS repeat_state,
    (SELECT r.frequency_anchor_date FROM task_schedules r WHERE r.id = ts.series_id) AS frequency_anchor_date,
    ts.series_id,
    ts.occurrence_date,
    ts.timezone,
    ts.is_exception,
    ts.completed,
    (ts.deleted_at IS NOT NULL) AS deleted,
    ARRAY(
        SELECT tsf.frequency
        FROM task_schedule_frequencies AS tsf
        WHERE tsf.task_schedule_id = ts.series_id
        ORDER BY tsf.frequency
    )::text[] AS frequencies,
    ts.start_at,
    ts.end_at,
    ts.created_at,
    ts.updated_at
FROM task_schedules AS ts
JOIN tasks AS t ON t.id = ts.task_id
WHERE t.user_id = $1
AND t.deleted_at IS NULL
AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND t.status <> 'done'
  AND ts.id = ts.series_id
  AND EXISTS (SELECT 1 FROM task_schedules r WHERE r.id = ts.series_id AND r.repeat_state = 'active')
  AND ts.deleted_at IS NULL
ORDER BY ts.start_at ASC;
