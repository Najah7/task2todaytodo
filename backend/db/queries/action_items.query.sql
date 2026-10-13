-- name: GetActionItemByTaskAndUserID :one
SELECT ai.id, ai.task_id, ai.title, ai.description, ai.due_date, ai.estimated_minutes, ai.actual_minutes,
       EXISTS (SELECT 1 FROM todo_items ti WHERE ti.action_item_id = ai.id AND ti.kind = 'action_item' AND ti.completed) AS started,
       NOT ai.completed AND NOT EXISTS (SELECT 1 FROM todo_items ti WHERE ti.action_item_id = ai.id AND ti.kind = 'action_item' AND ti.completed) AS estimate_editable,
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
SELECT ai.id, ai.task_id, ai.title, ai.description, ai.due_date, ai.estimated_minutes, ai.actual_minutes,
       EXISTS (SELECT 1 FROM todo_items ti WHERE ti.action_item_id = ai.id AND ti.kind = 'action_item' AND ti.completed) AS started,
       NOT ai.completed AND NOT EXISTS (SELECT 1 FROM todo_items ti WHERE ti.action_item_id = ai.id AND ti.kind = 'action_item' AND ti.completed) AS estimate_editable,
       ai.priority, ai.completed, ai.position, (ai.deleted_at IS NOT NULL) AS deleted,
       ai.created_at, ai.updated_at
FROM action_items ai JOIN tasks t ON t.id = ai.task_id
WHERE ai.id = sqlc.arg(id)::text
  AND ai.task_id = sqlc.arg(task_id)::text
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(actor_id)::text, sqlc.arg(resource_id)::text, sqlc.arg(action)::action);

-- name: ListActionItemsByTaskAndUserID :many
SELECT ai.id, ai.task_id, ai.title, ai.description, ai.due_date, ai.estimated_minutes, ai.actual_minutes,
       EXISTS (SELECT 1 FROM todo_items ti WHERE ti.action_item_id = ai.id AND ti.kind = 'action_item' AND ti.completed) AS started,
       NOT ai.completed AND NOT EXISTS (SELECT 1 FROM todo_items ti WHERE ti.action_item_id = ai.id AND ti.kind = 'action_item' AND ti.completed) AS estimate_editable,
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
SELECT ai.id, ai.task_id, ai.title, ai.description, ai.due_date, ai.estimated_minutes, ai.actual_minutes,
       EXISTS (SELECT 1 FROM todo_items ti WHERE ti.action_item_id = ai.id AND ti.kind = 'action_item' AND ti.completed) AS started,
       NOT ai.completed AND NOT EXISTS (SELECT 1 FROM todo_items ti WHERE ti.action_item_id = ai.id AND ti.kind = 'action_item' AND ti.completed) AS estimate_editable,
       ai.priority, ai.completed, ai.position, (ai.deleted_at IS NOT NULL) AS deleted,
       ai.created_at, ai.updated_at
FROM action_items ai JOIN tasks t ON t.id = ai.task_id
WHERE ai.task_id = ANY(sqlc.arg(task_ids)::text[])
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'read')
  AND ai.deleted_at IS NULL
ORDER BY ai.task_id, ai.position, ai.id;

-- name: ListTasksWithCompletedTodoFrames :many
SELECT DISTINCT ti.task_id
FROM todo_items ti
JOIN tasks t ON t.id = ti.task_id
WHERE ti.kind = 'task'
  AND ti.task_id = ANY(sqlc.arg(task_ids)::text[])
  AND ti.completed = true
  AND t.deleted_at IS NULL
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'task', 'read');

-- name: ListTasksWithActionItemChildren :many
SELECT DISTINCT t.id AS task_id
FROM tasks t
JOIN action_items ai ON ai.task_id = t.id AND ai.deleted_at IS NULL
WHERE t.id = ANY(sqlc.arg(task_ids)::text[])
  AND t.deleted_at IS NULL
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'task', 'read');

-- name: ListTasksWithActionItemCreatePermission :many
SELECT t.id AS task_id
FROM tasks t
WHERE t.id = ANY(sqlc.arg(task_ids)::text[])
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL AND p.status <> 'done'))
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'create');

-- name: CountActionItemsForTaskCommand :one
SELECT count(*)::integer AS child_count
FROM action_items ai
JOIN tasks t ON t.id = ai.task_id
WHERE ai.task_id = sqlc.arg(task_id)::text
  AND ai.deleted_at IS NULL
  AND t.deleted_at IS NULL
  AND task_has_permission(t.id, sqlc.arg(actor_id)::text, sqlc.arg(resource_id)::text, sqlc.arg(action)::action);

-- name: CountActionItemsForProjectPlanLimit :one
SELECT count(*)::integer AS item_count
FROM action_items ai
JOIN tasks t ON t.id = ai.task_id
WHERE t.project_id = sqlc.arg(project_id)::text
  AND t.deleted_at IS NULL
  AND ai.deleted_at IS NULL;

-- name: CountProjectlessActionItemsForUserPlanLimit :one
SELECT count(*)::integer AS item_count
FROM action_items ai
JOIN tasks t ON t.id = ai.task_id
WHERE t.user_id = sqlc.arg(user_id)::text
  AND t.project_id IS NULL
  AND t.deleted_at IS NULL
  AND ai.deleted_at IS NULL;

-- name: CountActionItemsForTaskPlanLimit :one
SELECT count(*)::integer AS item_count
FROM action_items ai
JOIN tasks t ON t.id = ai.task_id
WHERE t.id = sqlc.arg(task_id)::text
  AND t.deleted_at IS NULL
  AND ai.deleted_at IS NULL;

-- name: ListActionItemsByTaskAndUserIDCursorPage :many
SELECT ai.id, ai.task_id, ai.title, ai.description, ai.due_date, ai.estimated_minutes, ai.actual_minutes,
       EXISTS (SELECT 1 FROM todo_items ti WHERE ti.action_item_id = ai.id AND ti.kind = 'action_item' AND ti.completed) AS started,
       NOT ai.completed AND NOT EXISTS (SELECT 1 FROM todo_items ti WHERE ti.action_item_id = ai.id AND ti.kind = 'action_item' AND ti.completed) AS estimate_editable,
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
