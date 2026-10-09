-- name: GetTask :one
SELECT *
FROM tasks
WHERE id = $1;

-- name: GetTaskByUserID :one
SELECT *
FROM tasks AS t
WHERE t.id = $1
  AND task_has_permission(t.id, $2, 'task', 'read');

-- name: GetTaskByUserIDForPermission :one
SELECT *
FROM tasks AS t
WHERE t.id = sqlc.arg(id)::text
  AND task_has_permission(t.id, sqlc.arg(actor_id)::text, sqlc.arg(resource_id)::text, sqlc.arg(action)::action);

-- name: ListTasksByUserIDCursorPage :many
SELECT t.*
FROM tasks AS t
WHERE t.assignee_id = sqlc.arg(user_id)
  AND t.deleted_at IS NULL
  AND task_has_permission(t.id, sqlc.arg(user_id), 'task', 'read')
  AND (sqlc.narg(cursor_at)::timestamptz IS NULL OR (created_at, id) < (sqlc.narg(cursor_at)::timestamptz, sqlc.narg(cursor_id)::text))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_limit)::integer;

-- name: ListUsersWithActiveRecurrences :many
SELECT DISTINCT t.user_id
FROM tasks AS t
WHERE t.status <> 'done'
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id=t.project_id AND p.deleted_at IS NULL AND p.status <> 'done'))
  AND EXISTS (
      SELECT 1 FROM action_items AS ti
      WHERE ti.task_id = t.id AND ti.id = ti.series_id
        AND ti.repeat_state = 'active' AND ti.deleted_at IS NULL
  )
ORDER BY t.user_id;

-- name: ListProjectTasksByUserIDCursorPage :many
SELECT t.*
FROM tasks AS t
JOIN projects AS p ON p.id = t.project_id AND p.user_id = t.user_id AND p.deleted_at IS NULL
WHERE t.project_id = sqlc.arg(project_id)
  AND t.deleted_at IS NULL
  AND task_has_permission(t.id, sqlc.arg(user_id), 'task', 'read')
  AND (sqlc.narg(cursor_at)::timestamptz IS NULL OR (t.created_at, t.id) < (sqlc.narg(cursor_at)::timestamptz, sqlc.narg(cursor_id)::text))
ORDER BY t.created_at DESC, t.id DESC
LIMIT sqlc.arg(page_limit)::integer;

-- name: ListTasksByProjectAndUserID :many
SELECT t.id, t.user_id, t.project_id, t.title, t.description, t.due_date, t.estimated_minutes, t.actual_minutes,
       t.priority, t.status, t.created_at, t.updated_at
FROM tasks AS t
JOIN projects AS p ON p.id = t.project_id
WHERE t.project_id = $1
  AND p.user_id = $2
ORDER BY t.created_at ASC;

-- name: ListActionItemsByTaskForUser :many
SELECT
    ti.id,
    ti.task_id,
    ti.title,
    ti.description,
    ti.due_date,
    ti.completed,
    ti.position,
    COALESCE((SELECT r.interval_weeks FROM action_items r WHERE r.id = ti.series_id ), 0)::integer AS interval_weeks,
    ARRAY(
        SELECT tif.frequency
        FROM action_item_frequencies AS tif
        WHERE tif.action_item_id = ti.series_id
        ORDER BY tif.frequency
    )::text[] AS frequencies,
    ti.created_at,
    ti.updated_at
FROM action_items AS ti
JOIN tasks AS t ON t.id = ti.task_id
WHERE ti.task_id = $1
  AND t.user_id = $2
	AND ti.deleted_at IS NULL
ORDER BY ti.position ASC;

-- name: GetTaskByTag :many
SELECT t.*
FROM tasks AS t
JOIN task_tag_assignments AS tta ON tta.task_id = t.id
WHERE tta.tag_id = $1
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id=t.project_id AND p.deleted_at IS NULL));

-- name: GetTaskByStatus :many
SELECT * FROM tasks WHERE tasks.status = $1 AND deleted_at IS NULL
  AND (project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id=tasks.project_id AND p.deleted_at IS NULL));

-- name: GetTaskByPriority :many
SELECT * FROM tasks WHERE tasks.priority = $1 AND deleted_at IS NULL
  AND (project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id=tasks.project_id AND p.deleted_at IS NULL));

-- name: GetTaskByProject :many
SELECT * FROM tasks WHERE project_id = sqlc.arg(project_id)::text AND deleted_at IS NULL
  AND EXISTS (SELECT 1 FROM projects p WHERE p.id=tasks.project_id AND p.deleted_at IS NULL);

-- name: GetTaskByFrequency :many
SELECT t.*
FROM tasks AS t
WHERE t.deleted_at IS NULL
AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id=t.project_id AND p.deleted_at IS NULL))
AND EXISTS (
    SELECT 1
    FROM action_items AS ti
    JOIN action_item_frequencies AS tif ON tif.action_item_id = ti.series_id
    WHERE ti.task_id = t.id
      AND tif.frequency = sqlc.arg(frequency)::text
);
