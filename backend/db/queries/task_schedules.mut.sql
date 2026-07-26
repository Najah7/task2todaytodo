-- name: CreateTaskSchedule :one
INSERT INTO task_schedules (id, task_id, title, description, location, interval_weeks, start_at, end_at, due_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, task_id, title, description, location, interval_weeks, start_at, end_at, due_at, created_at, updated_at;

-- name: CreateTaskScheduleByTaskAndUser :one
WITH inserted AS (
    INSERT INTO task_schedules (id, task_id, title, description, location, interval_weeks, start_at, end_at, due_at)
    SELECT
        sqlc.arg(id),
        sqlc.arg(task_id),
        sqlc.arg(title),
        sqlc.arg(description),
        sqlc.arg(location),
        sqlc.arg(interval_weeks),
        sqlc.arg(start_at),
        sqlc.arg(end_at),
        sqlc.arg(due_at)
    FROM tasks AS t
    WHERE t.id = sqlc.arg(task_id)
      AND t.user_id = sqlc.arg(user_id)
    RETURNING id, task_id, title, description, location, interval_weeks, start_at, end_at, due_at, created_at, updated_at
),
inserted_frequencies AS (
    INSERT INTO task_schedule_frequencies (task_schedule_id, frequency)
    SELECT inserted.id, unnest(sqlc.arg(frequencies)::text[])
    FROM inserted
    ON CONFLICT DO NOTHING
)
SELECT
    inserted.id,
    inserted.task_id,
    inserted.title,
    inserted.description,
    inserted.location,
    inserted.interval_weeks,
    ARRAY(
        SELECT tsf.frequency
        FROM task_schedule_frequencies AS tsf
        WHERE tsf.task_schedule_id = inserted.id
        ORDER BY tsf.frequency
    )::text[] AS frequencies,
    inserted.start_at,
    inserted.end_at,
    inserted.due_at,
    inserted.created_at,
    inserted.updated_at
FROM inserted;

-- name: UpdateTaskSchedule :one
UPDATE task_schedules
SET task_id = $2,
    title = $3,
    description = $4,
    location = $5,
    interval_weeks = $6,
    start_at = $7,
    end_at = $8,
    due_at = $9,
    updated_at = now()
WHERE id = $1
RETURNING id, task_id, title, description, location, interval_weeks, start_at, end_at, due_at, created_at, updated_at;

-- name: UpdateTaskScheduleByTaskAndUser :one
WITH updated AS (
    UPDATE task_schedules AS ts
    SET title = sqlc.arg(title),
        description = sqlc.arg(description),
        location = sqlc.arg(location),
        interval_weeks = sqlc.arg(interval_weeks),
        start_at = sqlc.arg(start_at),
        end_at = sqlc.arg(end_at),
        due_at = sqlc.arg(due_at),
        updated_at = now()
    FROM tasks AS t
    WHERE ts.id = sqlc.arg(id)
      AND ts.task_id = sqlc.arg(task_id)
      AND t.id = ts.task_id
      AND t.user_id = sqlc.arg(user_id)
    RETURNING ts.id, ts.task_id, ts.title, ts.description, ts.location, ts.interval_weeks, ts.start_at, ts.end_at, ts.due_at, ts.created_at, ts.updated_at
),
deleted_frequencies AS (
    DELETE FROM task_schedule_frequencies AS tsf
    USING updated
    WHERE tsf.task_schedule_id = updated.id
    RETURNING tsf.task_schedule_id
),
inserted_frequencies AS (
    INSERT INTO task_schedule_frequencies (task_schedule_id, frequency)
    SELECT updated.id, unnest(sqlc.arg(frequencies)::text[])
    FROM updated
    LEFT JOIN (SELECT count(*) AS deleted_count FROM deleted_frequencies) AS deleted ON true
    ON CONFLICT DO NOTHING
)
SELECT
    updated.id,
    updated.task_id,
    updated.title,
    updated.description,
    updated.location,
    updated.interval_weeks,
    ARRAY(
        SELECT tsf.frequency
        FROM task_schedule_frequencies AS tsf
        WHERE tsf.task_schedule_id = updated.id
        ORDER BY tsf.frequency
    )::text[] AS frequencies,
    updated.start_at,
    updated.end_at,
    updated.due_at,
    updated.created_at,
    updated.updated_at
FROM updated;

-- name: DeleteTaskSchedule :exec
DELETE FROM task_schedules
WHERE id = $1;

-- name: DeleteTaskScheduleByTaskAndUser :one
DELETE FROM task_schedules AS ts
USING tasks AS t
WHERE ts.id = $1
  AND ts.task_id = $2
  AND t.id = ts.task_id
  AND t.user_id = $3
RETURNING ts.id;

-- name: DeleteTaskScheduleFrequencies :exec
DELETE FROM task_schedule_frequencies
WHERE task_schedule_id = $1;

-- name: AddTaskScheduleFrequency :exec
INSERT INTO task_schedule_frequencies (task_schedule_id, frequency)
VALUES ($1, $2);
