-- name: GetProject :one
SELECT *
FROM projects
WHERE id = $1;

-- name: GetProjectByUserID :one
SELECT *
FROM projects
WHERE id = $1
  AND deleted_at IS NULL
  AND project_has_permission(id, $2, 'project', 'read');

-- name: GetProjectByUserIDForPermission :one
SELECT p.*
FROM projects AS p
WHERE p.id = sqlc.arg(id)::text
  AND p.deleted_at IS NULL
  AND project_has_permission(p.id, sqlc.arg(actor_id)::text, sqlc.arg(resource_id)::text, sqlc.arg(action)::permission_action);

-- name: LockProjectByUserIDForPermission :one
SELECT p.*
FROM projects AS p
WHERE p.id = sqlc.arg(id)::text
  AND p.deleted_at IS NULL
  AND project_has_permission(p.id, sqlc.arg(user_id)::text, sqlc.arg(resource_id)::text, sqlc.arg(action)::permission_action)
FOR UPDATE;

-- name: LockActiveProjectTasksForDeletion :many
SELECT id
FROM tasks
WHERE project_id = sqlc.arg(project_id)::text
  AND deleted_at IS NULL
ORDER BY id
FOR UPDATE;

-- name: ListProjectsByUserID :many
SELECT *
FROM projects
WHERE deleted_at IS NULL
  AND project_has_permission(projects.id, sqlc.arg(user_id)::text, 'project', 'read')
ORDER BY created_at DESC, id DESC;

-- name: ListProjectsByUserIDPage :many
SELECT *
FROM projects
WHERE deleted_at IS NULL
  AND project_has_permission(projects.id, sqlc.arg(user_id)::text, 'project', 'read')
  AND (sqlc.narg(cursor_at)::timestamptz IS NULL OR (created_at, id) < (sqlc.narg(cursor_at)::timestamptz, sqlc.narg(cursor_id)::text))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_limit)::integer;

-- name: ListProjectTasksByUserID :many
SELECT t.*
FROM tasks AS t
WHERE t.project_id = $1
  AND t.deleted_at IS NULL
  AND task_has_permission(t.id, $2, 'task', 'read')
  AND EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL)
ORDER BY t.created_at ASC;
