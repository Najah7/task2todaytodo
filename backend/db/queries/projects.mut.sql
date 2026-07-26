-- name: CreateProject :one
INSERT INTO projects (id, user_id, type, title, goal, description, due_date, progress, priority, start_at, end_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING id, user_id, type, title, goal, description, due_date, progress, priority, start_at, end_at, created_at, updated_at;

-- name: UpdateProject :one
UPDATE projects
SET type = $2,
    title = $3,
    goal = $4,
    description = $5,
    due_date = $6,
    progress = $7,
    priority = $8,
    start_at = $9,
    end_at = $10,
    updated_at = now()
WHERE id = $1
RETURNING id, user_id, type, title, goal, description, due_date, progress, priority, start_at, end_at, created_at, updated_at;

-- name: UpdateProjectByUser :one
UPDATE projects
SET type = $3,
    title = $4,
    goal = $5,
    description = $6,
    due_date = $7,
    progress = $8,
    priority = $9,
    start_at = $10,
    end_at = $11,
    updated_at = now()
WHERE id = $1
  AND user_id = $2
RETURNING id, user_id, type, title, goal, description, due_date, progress, priority, start_at, end_at, created_at, updated_at;

-- name: DeleteProject :exec
DELETE FROM projects
WHERE id = $1;

-- name: DeleteProjectByUser :execrows
DELETE FROM projects
WHERE id = $1
  AND user_id = $2;
