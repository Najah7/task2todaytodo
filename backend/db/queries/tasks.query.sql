-- name: GetTask :one
SELECT id, user_id, project_id, title, description, estimated_minutes, actual_minutes, progress,
       priority, status, created_at, updated_at
FROM tasks
WHERE id = $1;

-- name: GetTaskByUser :one
SELECT id, user_id, project_id, title, description, estimated_minutes, actual_minutes, progress,
       priority, status, created_at, updated_at
FROM tasks
WHERE id = $1
  AND user_id = $2;

-- name: ListTasksByProjectAndUser :many
SELECT t.id, t.user_id, t.project_id, t.title, t.description, t.estimated_minutes, t.actual_minutes, t.progress,
       t.priority, t.status, t.created_at, t.updated_at
FROM tasks AS t
JOIN projects AS p ON p.id = t.project_id
WHERE t.project_id = $1
  AND p.user_id = $2
ORDER BY t.created_at ASC;

-- name: ListTodoItemsByTaskForUser :many
SELECT
    ti.id,
    ti.task_id,
    ti.title,
    ti.description,
    ti.completed,
    ti.position,
    ti.interval_weeks,
    ARRAY(
        SELECT tif.frequency
        FROM todo_item_frequencies AS tif
        WHERE tif.todo_item_id = ti.id
        ORDER BY tif.frequency
    )::text[] AS frequencies,
    ti.created_at,
    ti.updated_at
FROM todo_items AS ti
JOIN tasks AS t ON t.id = ti.task_id
WHERE ti.task_id = $1
  AND t.user_id = $2
ORDER BY ti.position ASC;

-- name: ListTaskSchedulesByTaskForUser :many
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

-- name: GetTaskByTag :many
SELECT t.id, t.user_id, t.project_id, t.title, t.description, t.estimated_minutes, t.actual_minutes, t.progress,
       t.priority, t.status, t.created_at, t.updated_at
FROM tasks AS t
JOIN task_tag_assignments AS tta ON tta.task_id = t.id
WHERE tta.tag_id = $1;

-- name: GetTaskByStatus :many
SELECT id, user_id, project_id, title, description, estimated_minutes, actual_minutes, progress,
       priority, status, created_at, updated_at
FROM tasks
WHERE status = $1;

-- name: GetTaskByPriority :many
SELECT id, user_id, project_id, title, description, estimated_minutes, actual_minutes, progress,
       priority, status, created_at, updated_at
FROM tasks
WHERE priority = $1;

-- name: GetTaskByProject :many
SELECT id, user_id, project_id, title, description, estimated_minutes, actual_minutes, progress,
       priority, status, created_at, updated_at
FROM tasks
WHERE project_id = sqlc.arg(project_id)::text;

-- name: GetTaskByProjectType :many
SELECT t.id, t.user_id, t.project_id, t.title, t.description, t.estimated_minutes, t.actual_minutes, t.progress,
       t.priority, t.status, t.created_at, t.updated_at
FROM tasks AS t
JOIN projects AS p ON p.id = t.project_id
WHERE p.type = $1;

-- name: GetTaskByFrequency :many
SELECT t.id, t.user_id, t.project_id, t.title, t.description, t.estimated_minutes, t.actual_minutes, t.progress,
       t.priority, t.status, t.created_at, t.updated_at
FROM tasks AS t
WHERE EXISTS (
    SELECT 1
    FROM todo_items AS ti
    JOIN todo_item_frequencies AS tif ON tif.todo_item_id = ti.id
    WHERE ti.task_id = t.id
      AND tif.frequency = sqlc.arg(frequency)::text
)
OR EXISTS (
    SELECT 1
    FROM task_schedules AS ts
    JOIN task_schedule_frequencies AS tsf ON tsf.task_schedule_id = ts.id
    WHERE ts.task_id = t.id
      AND tsf.frequency = sqlc.arg(frequency)::text
);
