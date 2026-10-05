-- name: GetTaskTagByUserID :one
SELECT id, user_id, name, created_at, updated_at
FROM task_tags
WHERE id = sqlc.arg(id)::text
  AND user_id = sqlc.arg(user_id)::text;

-- name: ListTaskTagsByUserID :many
SELECT id, user_id, name, created_at, updated_at
FROM task_tags
WHERE user_id = sqlc.arg(user_id)::text
ORDER BY name ASC, id ASC;

-- name: ListTaskTagsByUserIDPage :many
SELECT id, user_id, name, created_at, updated_at
FROM task_tags
WHERE user_id = sqlc.arg(user_id)::text
  AND (sqlc.narg(cursor_name)::citext IS NULL OR (name, id) > (sqlc.narg(cursor_name)::citext, sqlc.narg(cursor_id)::text))
ORDER BY name ASC, id ASC
LIMIT sqlc.arg(page_limit)::integer;

-- name: ListTaskTagsByTaskAndUserID :many
SELECT tt.id, tt.user_id, tt.name, tt.created_at, tt.updated_at
FROM task_tags AS tt
JOIN task_tag_assignments AS tta ON tta.tag_id = tt.id
JOIN tasks AS t ON t.id = tta.task_id
WHERE t.id = sqlc.arg(task_id)::text
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'task', 'read')
  AND tt.user_id = t.user_id
ORDER BY tt.name ASC, tt.id ASC;
