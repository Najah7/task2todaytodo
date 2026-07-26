-- name: GetProject :one
SELECT id, user_id, type, title, goal, description, due_date, progress, priority, start_at, end_at, created_at, updated_at
FROM projects
WHERE id = $1;

-- name: GetProjectByUser :one
SELECT id, user_id, type, title, goal, description, due_date, progress, priority, start_at, end_at, created_at, updated_at
FROM projects
WHERE id = $1
  AND user_id = $2;

-- name: ListProjectTasksByUser :many
SELECT id, user_id, project_id, title, description, due_date, estimated_minutes, actual_minutes, progress,
       priority, status, created_at, updated_at
FROM tasks
WHERE project_id = $1
  AND user_id = $2
ORDER BY created_at ASC;
