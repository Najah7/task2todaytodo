-- name: GetTaskSchedule :one
SELECT
    ts.id,
    ts.task_id,
    ts.title,
    ts.description,
    ts.location,
    ts.interval_weeks,
    ARRAY(
        SELECT tsf.frequency
        FROM task_schedule_frequencies AS tsf
        WHERE tsf.task_schedule_id = ts.id
        ORDER BY tsf.frequency
    )::text[] AS frequencies,
    ts.start_at,
    ts.end_at,
    ts.due_at,
    ts.created_at,
    ts.updated_at
FROM task_schedules AS ts
WHERE ts.id = $1;

-- name: CreateTaskSchedule :one
INSERT INTO task_schedules (id, task_id, title, description, location, interval_weeks, start_at, end_at, due_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, task_id, title, description, location, interval_weeks, start_at, end_at, due_at, created_at, updated_at;

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

-- name: DeleteTaskSchedule :exec
DELETE FROM task_schedules
WHERE id = $1;
