-- name: GetActionItemByTaskAndUserID :one
SELECT ai.id, ai.task_id, ai.title, ai.description, ai.due_date, ai.estimated_minutes,
       ai.priority, ai.completed, ai.position, (ai.deleted_at IS NOT NULL) AS deleted,
       ai.created_at, ai.updated_at
FROM action_items ai JOIN tasks t ON t.id = ai.task_id
WHERE ai.id = sqlc.arg(id)::text
  AND ai.task_id = sqlc.arg(task_id)::text
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'read')
  AND ai.deleted_at IS NULL;

-- name: GetActionItemByTaskAndUserIDForCommand :one
SELECT ai.id, ai.task_id, ai.title, ai.description, ai.due_date, ai.estimated_minutes,
       ai.priority, ai.completed, ai.position, (ai.deleted_at IS NOT NULL) AS deleted,
       ai.created_at, ai.updated_at
FROM action_items ai JOIN tasks t ON t.id = ai.task_id
WHERE ai.id = sqlc.arg(id)::text
  AND ai.task_id = sqlc.arg(task_id)::text
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(actor_id)::text, sqlc.arg(resource_id)::text, sqlc.arg(action)::action);

-- name: ListActionItemsByTaskAndUserID :many
SELECT ai.id, ai.task_id, ai.title, ai.description, ai.due_date, ai.estimated_minutes,
       ai.priority, ai.completed, ai.position, (ai.deleted_at IS NOT NULL) AS deleted,
       ai.created_at, ai.updated_at
FROM action_items ai JOIN tasks t ON t.id = ai.task_id
WHERE ai.task_id = sqlc.arg(task_id)::text
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'read')
  AND ai.deleted_at IS NULL
ORDER BY ai.position ASC, ai.id ASC;

-- name: ListActionItemsForTaskIDsAndUserID :many
SELECT ai.id, ai.task_id, ai.title, ai.description, ai.due_date, ai.estimated_minutes,
       ai.priority, ai.completed, ai.position, (ai.deleted_at IS NOT NULL) AS deleted,
       ai.created_at, ai.updated_at
FROM action_items ai JOIN tasks t ON t.id = ai.task_id
WHERE ai.task_id = ANY(sqlc.arg(task_ids)::text[])
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'read')
  AND ai.deleted_at IS NULL
ORDER BY ai.task_id, ai.position, ai.id;

-- name: ListActionItemsByTaskAndUserIDCursorPage :many
SELECT ai.id, ai.task_id, ai.title, ai.description, ai.due_date, ai.estimated_minutes,
       ai.priority, ai.completed, ai.position, (ai.deleted_at IS NOT NULL) AS deleted,
       ai.created_at, ai.updated_at
FROM action_items ai JOIN tasks t ON t.id = ai.task_id
WHERE ai.task_id = sqlc.arg(task_id)::text
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'read')
  AND ai.deleted_at IS NULL
  AND (sqlc.narg(cursor_position)::integer IS NULL
       OR (ai.position, ai.id) > (sqlc.narg(cursor_position)::integer, sqlc.narg(cursor_id)::text))
ORDER BY ai.position ASC, ai.id ASC
LIMIT sqlc.arg(page_limit)::integer;

-- name: NextActionItemPosition :one
SELECT COALESCE(MAX(ai.position), -1)::integer + 1 AS position
FROM action_items ai JOIN tasks t ON t.id = ai.task_id
WHERE ai.task_id = sqlc.arg(task_id)::text
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'create')
  AND ai.deleted_at IS NULL;
