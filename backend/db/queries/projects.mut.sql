-- name: CreateProject :one
INSERT INTO projects (id, user_id, type, title, goal, description, priority, start_date, end_date, changed_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $2)
RETURNING *;

-- name: UpdateProject :one
UPDATE projects
SET type = $2,
    title = $3,
    goal = $4,
    description = $5,
    priority = $6,
    start_date = $7,
    end_date = $8,
    changed_by = $9
WHERE id = $1
RETURNING *;

-- name: UpdateProjectByUserID :one
UPDATE projects
SET type = $3,
    title = $4,
    goal = $5,
    description = $6,
    priority = $7,
    start_date = $8,
    end_date = $9,
    changed_by = $2
WHERE id = $1
  AND deleted_at IS NULL
  AND revision = sqlc.arg(expected_revision)::integer
  AND project_has_permission(id, sqlc.arg(user_id)::text, 'project', 'update')
RETURNING *;

-- name: DeleteProject :execrows
UPDATE projects
SET deleted_at = now(), changed_by = user_id
WHERE id = $1 AND deleted_at IS NULL;

-- name: DeleteProjectByUserID :one
WITH deleted_project AS (
    UPDATE projects
    SET deleted_at = now(), changed_by = sqlc.arg(user_id)::text
    WHERE id = sqlc.arg(id)::text
      AND deleted_at IS NULL
      AND revision = sqlc.arg(expected_revision)::integer
      AND project_has_permission(id, sqlc.arg(user_id)::text, 'project', 'delete')
    RETURNING id
)
SELECT id FROM deleted_project;

-- name: SetProjectStatusByUserID :one
UPDATE projects
SET status = sqlc.arg(status)::text,
    changed_by = sqlc.arg(user_id)::text
WHERE id = sqlc.arg(id)::text
  AND deleted_at IS NULL
  AND revision = sqlc.arg(expected_revision)::integer
  AND project_has_permission(id, sqlc.arg(user_id)::text, 'project', 'update')
RETURNING *;

-- name: RestoreProjectByUserID :one
UPDATE projects
SET deleted_at = NULL,
    changed_by = sqlc.arg(user_id)::text
WHERE id = sqlc.arg(id)::text
  AND deleted_at IS NOT NULL
  AND revision = sqlc.arg(expected_revision)::integer
  AND project_has_permission(id, sqlc.arg(user_id)::text, 'project', 'delete')
RETURNING *;

-- name: SetProjectStatusForLifecycle :one
UPDATE projects
SET status = sqlc.arg(status)::text,
    changed_by = sqlc.arg(actor_id)::text
WHERE id = sqlc.arg(id)::text
  AND deleted_at IS NULL
  AND status <> sqlc.arg(status)::text
RETURNING *;
