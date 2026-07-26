
-- name: CreateTask :one
INSERT INTO tasks (
    id, user_id, project_id, title, description, due_date, estimated_minutes, actual_minutes,
    progress, priority, status
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING id, user_id, project_id, title, description, due_date, estimated_minutes, actual_minutes,
          progress, priority, status, created_at, updated_at;

-- name: CreateTaskInProject :one
INSERT INTO tasks (
    id, user_id, project_id, title, description, due_date, estimated_minutes, actual_minutes,
    progress, priority, status
)
SELECT
    sqlc.arg(id),
    sqlc.arg(user_id),
    p.id,
    sqlc.arg(title),
    sqlc.arg(description),
    sqlc.arg(due_date),
    sqlc.arg(estimated_minutes),
    sqlc.arg(actual_minutes),
    sqlc.arg(progress),
    sqlc.arg(priority),
    sqlc.arg(status)
FROM projects AS p
WHERE p.id = sqlc.arg(project_id)
  AND p.user_id = sqlc.arg(user_id)
RETURNING id, user_id, project_id, title, description, due_date, estimated_minutes, actual_minutes,
          progress, priority, status, created_at, updated_at;

-- name: UpdateTask :one
UPDATE tasks
SET user_id = $2,
    project_id = $3,
    title = $4,
    description = $5,
    due_date = $6,
    estimated_minutes = $7,
    actual_minutes = $8,
    progress = $9,
    priority = $10,
    status = $11,
    updated_at = now()
WHERE id = $1
RETURNING id, user_id, project_id, title, description, due_date, estimated_minutes, actual_minutes,
          progress, priority, status, created_at, updated_at;

-- name: UpdateTaskByUser :one
UPDATE tasks
SET project_id = $3,
    title = $4,
    description = $5,
    due_date = $6,
    estimated_minutes = $7,
    actual_minutes = $8,
    progress = $9,
    priority = $10,
    status = $11,
    updated_at = now()
WHERE id = $1
  AND user_id = $2
RETURNING id, user_id, project_id, title, description, due_date, estimated_minutes, actual_minutes,
          progress, priority, status, created_at, updated_at;

-- name: DeleteTask :exec
DELETE FROM tasks
WHERE id = $1;

-- name: DeleteTaskByUser :one
DELETE FROM tasks
WHERE id = $1
  AND user_id = $2
RETURNING id;
