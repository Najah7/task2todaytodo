-- name: CreateTaskSchedule :one
WITH inserted AS (
    INSERT INTO task_schedules (id, task_id, title, description, location, series_id, occurrence_date, timezone, is_exception, start_at, end_at, repeat_state, frequency_anchor_date, interval_weeks)
    VALUES (sqlc.arg(id), sqlc.arg(task_id), sqlc.arg(title), sqlc.narg(description), sqlc.narg(location),
        sqlc.arg(series_id), sqlc.arg(occurrence_date), sqlc.arg(timezone), sqlc.arg(is_exception), sqlc.arg(start_at), sqlc.arg(end_at),
        CASE WHEN sqlc.arg(interval_weeks)::integer > 0 THEN 'active' ELSE 'one_off' END,
        CASE WHEN sqlc.arg(interval_weeks)::integer > 0 THEN sqlc.arg(occurrence_date)::date ELSE NULL END,
        sqlc.arg(interval_weeks)::integer)
    RETURNING id, task_id, title, description, location, series_id, occurrence_date, timezone, is_exception, completed,
        (deleted_at IS NOT NULL) AS deleted, start_at, end_at, created_at, updated_at
)
SELECT inserted.*, sqlc.arg(interval_weeks)::integer AS interval_weeks FROM inserted;

-- name: CreateTaskScheduleByTaskAndUserID :one
WITH inserted AS (
    INSERT INTO task_schedules (id, task_id, title, description, location, series_id, occurrence_date, timezone, is_exception, start_at, end_at, repeat_state, frequency_anchor_date, interval_weeks)
    SELECT
        sqlc.arg(id),
        sqlc.arg(task_id),
        sqlc.arg(title),
        sqlc.arg(description),
        sqlc.arg(location),
        sqlc.arg(series_id),
        sqlc.arg(occurrence_date),
        sqlc.arg(timezone),
        sqlc.arg(is_exception),
        sqlc.arg(start_at),
        sqlc.arg(end_at),
        CASE WHEN sqlc.arg(interval_weeks)::integer > 0 THEN 'active' ELSE 'one_off' END,
        CASE WHEN sqlc.arg(interval_weeks)::integer > 0 THEN sqlc.arg(occurrence_date)::date ELSE NULL END,
        sqlc.arg(interval_weeks)::integer
    FROM tasks AS t
    WHERE t.id = sqlc.arg(task_id)
      AND t.deleted_at IS NULL
      AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
      AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'task_schedule', 'create')
    RETURNING id, task_id, title, description, location, series_id, occurrence_date, timezone, is_exception, interval_weeks, completed, (deleted_at IS NOT NULL) AS deleted, start_at, end_at, created_at, updated_at
),
inserted_frequencies AS (
    INSERT INTO task_schedule_frequencies (task_schedule_id, frequency)
    SELECT inserted.id, unnest(sqlc.arg(frequencies)::text[])
    FROM inserted
    WHERE inserted.id = inserted.series_id AND inserted.interval_weeks > 0
    ON CONFLICT DO NOTHING
)
SELECT
    inserted.id,
    inserted.task_id,
    inserted.title,
    inserted.description,
    inserted.location,
    sqlc.arg(interval_weeks)::integer AS interval_weeks,
    inserted.series_id,
    inserted.occurrence_date,
    inserted.timezone,
    inserted.is_exception,
    inserted.completed,
    false AS deleted,
    ARRAY(
        SELECT tsf.frequency
        FROM task_schedule_frequencies AS tsf
        WHERE tsf.task_schedule_id = inserted.id
        ORDER BY tsf.frequency
    )::text[] AS frequencies,
    inserted.start_at,
    inserted.end_at,
    inserted.created_at,
    inserted.updated_at
FROM inserted;

-- name: UpdateTaskSchedule :one
UPDATE task_schedules
SET task_id = $2,
    title = $3,
    description = $4,
    location = $5,
    start_at = $6,
    end_at = $7,
    updated_at = now()
WHERE task_schedules.id = $1
  AND task_schedules.deleted_at IS NULL
RETURNING id, task_id, title, description, location, COALESCE((SELECT r.interval_weeks FROM task_schedules r WHERE r.id = task_schedules.series_id ), 0)::integer AS interval_weeks, series_id, occurrence_date, timezone, is_exception, completed, (deleted_at IS NOT NULL) AS deleted, start_at, end_at, created_at, updated_at;

-- name: UpdateTaskScheduleByTaskAndUserID :one
WITH updated AS (
    UPDATE task_schedules AS ts
    SET title = sqlc.arg(title),
        description = sqlc.arg(description),
        location = sqlc.arg(location),
        start_at = sqlc.arg(start_at),
        end_at = sqlc.arg(end_at),
        is_exception = true,
        updated_at = now()
    FROM tasks AS t
    WHERE ts.id = sqlc.arg(id)
      AND ts.task_id = sqlc.arg(task_id)
      AND t.id = ts.task_id
      AND t.deleted_at IS NULL
      AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
      AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'task_schedule', 'update')
      AND ts.deleted_at IS NULL
    RETURNING ts.id, ts.task_id, ts.title, ts.description, ts.location, COALESCE((SELECT r.interval_weeks FROM task_schedules r WHERE r.id = ts.series_id ), 0)::integer AS interval_weeks, ts.series_id, ts.occurrence_date, ts.timezone, ts.is_exception, ts.completed, (ts.deleted_at IS NOT NULL) AS deleted, ts.start_at, ts.end_at, ts.created_at, ts.updated_at
)
SELECT
    updated.id,
    updated.task_id,
    updated.title,
    updated.description,
    updated.location,
    COALESCE((SELECT r.interval_weeks FROM task_schedules r WHERE r.id = updated.series_id ), 0)::integer AS interval_weeks,
    updated.series_id,
    updated.occurrence_date,
    updated.timezone,
    updated.is_exception,
    updated.completed,
    false AS deleted,
    ARRAY(
        SELECT tsf.frequency
        FROM task_schedule_frequencies AS tsf
        WHERE tsf.task_schedule_id = updated.series_id
        ORDER BY tsf.frequency
    )::text[] AS frequencies,
    updated.start_at,
    updated.end_at,
    updated.created_at,
    updated.updated_at
FROM updated;

-- name: DeleteTaskSchedule :exec
UPDATE task_schedules
SET deleted_at = COALESCE(deleted_at, now()), updated_at = now()
WHERE id = $1;

-- name: DeleteTaskScheduleByTaskAndUserID :one
UPDATE task_schedules AS ts
SET deleted_at = COALESCE(ts.deleted_at, now()), updated_at = now()
FROM tasks AS t
WHERE ts.id = $1
  AND ts.task_id = $2
  AND t.id = ts.task_id
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, $3, 'task_schedule', 'delete')
  AND ts.deleted_at IS NULL
RETURNING ts.id;

-- name: CreateTaskScheduleOccurrenceByTaskAndUserID :one
INSERT INTO task_schedules (
    id, task_id, title, description, location,
    series_id, occurrence_date, timezone, is_exception, start_at, end_at,
    repeat_state, frequency_anchor_date, interval_weeks
)
SELECT
    sqlc.arg(id), sqlc.arg(task_id),
    source.title,
    source.description,
    source.location,
    sqlc.arg(series_id),
    sqlc.arg(occurrence_date), sqlc.arg(timezone), sqlc.arg(is_exception),
    sqlc.arg(start_at), sqlc.arg(end_at), NULL, NULL, 0
FROM tasks AS t
JOIN task_schedules AS source ON source.id = sqlc.arg(series_id)
WHERE t.id = sqlc.arg(task_id)
  AND source.task_id = t.id
  AND source.series_id = source.id
  AND source.repeat_state = 'active'
  AND source.frequency_anchor_date <= sqlc.arg(occurrence_date)::date
  AND source.deleted_at IS NULL
  AND t.user_id = sqlc.arg(user_id)
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND t.status <> 'done'
ON CONFLICT (series_id, occurrence_date) WHERE id <> series_id DO UPDATE
SET id = task_schedules.id
WHERE task_schedules.deleted_at IS NULL
  AND task_schedules.task_id = sqlc.arg(task_id)
RETURNING id, task_id, title, description, location, COALESCE((SELECT r.interval_weeks FROM task_schedules r WHERE r.id = task_schedules.series_id ), 0)::integer AS interval_weeks,
    series_id, occurrence_date, timezone, is_exception, completed, (deleted_at IS NOT NULL) AS deleted,
    ARRAY(
        SELECT tsf.frequency
        FROM task_schedule_frequencies AS tsf
        WHERE tsf.task_schedule_id = task_schedules.series_id
        ORDER BY tsf.frequency
    )::text[] AS frequencies,
    start_at, end_at, created_at, updated_at;

-- name: TombstoneTaskScheduleByTaskAndUserID :one
UPDATE task_schedules AS ts
SET deleted_at = COALESCE(ts.deleted_at, now()), updated_at = now()
FROM tasks AS t
WHERE ts.id = sqlc.arg(id)
  AND ts.task_id = sqlc.arg(task_id)
  AND t.id = ts.task_id
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'task_schedule', 'delete')
  AND ts.deleted_at IS NULL
RETURNING ts.id;

-- name: SetTaskScheduleCompletedByTaskAndUserID :execrows
UPDATE task_schedules AS ts
SET completed = sqlc.arg(completed),
    is_exception = CASE WHEN ts.id = ts.series_id THEN ts.is_exception ELSE true END,
    updated_at = now()
FROM tasks AS t
WHERE ts.id = sqlc.arg(id)
  AND ts.task_id = sqlc.arg(task_id)
  AND t.id = ts.task_id
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'task_schedule', 'update')
  AND ts.deleted_at IS NULL;

-- name: DeleteUneditedFutureTaskSchedulesBySeries :execrows
UPDATE task_schedules AS ts
SET deleted_at = COALESCE(ts.deleted_at, now()), updated_at = now()
FROM tasks AS t
WHERE ts.task_id = sqlc.arg(task_id)
  AND t.id = ts.task_id
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'task_schedule', 'update')
  AND ts.series_id = sqlc.arg(series_id)
  AND ts.id <> ts.series_id
  AND ts.start_at > sqlc.arg(from_at)::timestamptz
  AND ts.is_exception = false
  AND ts.completed = false
  AND ts.deleted_at IS NULL;

-- name: DeleteUneditedFutureTaskSchedulesByTask :execrows
UPDATE task_schedules AS ts
SET deleted_at = COALESCE(ts.deleted_at, now()), updated_at = now()
FROM tasks AS t
WHERE ts.task_id = sqlc.arg(task_id)
  AND t.id = ts.task_id
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'task_schedule', 'update')
  AND ts.id <> ts.series_id
  AND ts.start_at > sqlc.arg(from_at)::timestamptz
  AND ts.is_exception = false
  AND ts.completed = false
  AND ts.deleted_at IS NULL;
