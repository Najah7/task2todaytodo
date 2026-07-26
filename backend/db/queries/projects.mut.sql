-- name: CreateProject :one
INSERT INTO projects (id, user_id, type, title, goal, description, progress, start_at, end_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, user_id, type, title, goal, description, progress, start_at, end_at, created_at, updated_at;

-- name: UpdateProject :one
UPDATE projects
SET type = $2,
    title = $3,
    goal = $4,
    description = $5,
    progress = $6,
    start_at = $7,
    end_at = $8,
    updated_at = now()
WHERE id = $1
RETURNING id, user_id, type, title, goal, description, progress, start_at, end_at, created_at, updated_at;

-- name: UpdateProjectByUser :one
UPDATE projects
SET type = $3,
    title = $4,
    goal = $5,
    description = $6,
    progress = $7,
    start_at = $8,
    end_at = $9,
    updated_at = now()
WHERE id = $1
  AND user_id = $2
RETURNING id, user_id, type, title, goal, description, progress, start_at, end_at, created_at, updated_at;

-- name: DeleteProject :exec
DELETE FROM projects
WHERE id = $1;

-- name: DeleteProjectByUser :execrows
DELETE FROM projects
WHERE id = $1
  AND user_id = $2;
