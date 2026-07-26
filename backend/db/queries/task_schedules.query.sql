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

-- name: GetTaskScheduleByTaskAndUser :one
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
JOIN tasks AS t ON t.id = ts.task_id
WHERE ts.id = $1
  AND ts.task_id = $2
  AND t.user_id = $3;

-- name: ListTaskSchedulesByTaskAndUser :many
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
JOIN tasks AS t ON t.id = ts.task_id
WHERE ts.task_id = $1
  AND t.user_id = $2
ORDER BY ts.start_at ASC;
