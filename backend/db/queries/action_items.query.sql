-- name: GetActionItem :one
SELECT
    ti.id,
    ti.task_id,
    ti.title,
    ti.description,
    ti.due_date,
    ti.completed,
    ti.position,
    COALESCE((SELECT r.interval_weeks FROM action_items r WHERE r.id = ti.series_id ), 0)::integer AS interval_weeks,
    (SELECT r.repeat_state FROM action_items r WHERE r.id = ti.series_id) AS repeat_state,
    (SELECT r.frequency_anchor_date FROM action_items r WHERE r.id = ti.series_id) AS frequency_anchor_date,
    ti.series_id,
    ti.occurrence_date,
    ti.timezone,
    ti.is_exception,
    (ti.deleted_at IS NOT NULL) AS deleted,
    ARRAY(
        SELECT tif.frequency
        FROM action_item_frequencies AS tif
        WHERE tif.action_item_id = ti.series_id
        ORDER BY tif.frequency
    )::text[] AS frequencies,
    ti.created_at,
    ti.updated_at
FROM action_items AS ti
WHERE ti.id = $1
  AND ti.deleted_at IS NULL;

-- name: GetActionItemByTaskAndUserID :one
SELECT
    ti.id,
    ti.task_id,
    ti.title,
    ti.description,
    ti.due_date,
    ti.completed,
    ti.position,
    COALESCE((SELECT r.interval_weeks FROM action_items r WHERE r.id = ti.series_id ), 0)::integer AS interval_weeks,
    (SELECT r.repeat_state FROM action_items r WHERE r.id = ti.series_id) AS repeat_state,
    (SELECT r.frequency_anchor_date FROM action_items r WHERE r.id = ti.series_id) AS frequency_anchor_date,
    ti.series_id,
    ti.occurrence_date,
    ti.timezone,
    ti.is_exception,
    (ti.deleted_at IS NOT NULL) AS deleted,
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
WHERE ti.id = $1
  AND ti.task_id = $2
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, $3, 'action_item', 'read')
  AND ti.deleted_at IS NULL;

-- name: GetActionItemByTaskAndUserIDForCommand :one
SELECT
    ti.id,
    ti.task_id,
    ti.title,
    ti.description,
    ti.due_date,
    ti.completed,
    ti.position,
    COALESCE((SELECT r.interval_weeks FROM action_items r WHERE r.id = ti.series_id), 0)::integer AS interval_weeks,
    (SELECT r.repeat_state FROM action_items r WHERE r.id = ti.series_id) AS repeat_state,
    (SELECT r.frequency_anchor_date FROM action_items r WHERE r.id = ti.series_id) AS frequency_anchor_date,
    ti.series_id,
    ti.occurrence_date,
    ti.timezone,
    ti.is_exception,
    (ti.deleted_at IS NOT NULL) AS deleted,
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
WHERE ti.id = sqlc.arg(id)::text
  AND ti.task_id = sqlc.arg(task_id)::text
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(actor_id)::text, sqlc.arg(resource_id)::text, sqlc.arg(action)::action);

-- name: ListActionItemsByTaskAndUserID :many
SELECT
    ti.id,
    ti.task_id,
    ti.title,
    ti.description,
    ti.due_date,
    ti.completed,
    ti.position,
    COALESCE((SELECT r.interval_weeks FROM action_items r WHERE r.id = ti.series_id ), 0)::integer AS interval_weeks,
    (SELECT r.repeat_state FROM action_items r WHERE r.id = ti.series_id) AS repeat_state,
    (SELECT r.frequency_anchor_date FROM action_items r WHERE r.id = ti.series_id) AS frequency_anchor_date,
    ti.series_id,
    ti.occurrence_date,
    ti.timezone,
    ti.is_exception,
    (ti.deleted_at IS NOT NULL) AS deleted,
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
WHERE ti.task_id = sqlc.arg(task_id)::text
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'read')
  AND ti.deleted_at IS NULL
ORDER BY ti.position ASC, ti.occurrence_date ASC;

-- name: ListActionItemsForOccurrenceProjectionByTaskAndUserID :many
SELECT
    ti.id,
    ti.task_id,
    ti.title,
    ti.description,
    ti.due_date,
    ti.completed,
    ti.position,
    COALESCE((SELECT r.interval_weeks FROM action_items r WHERE r.id = ti.series_id ), 0)::integer AS interval_weeks,
    (SELECT r.repeat_state FROM action_items r WHERE r.id = ti.series_id) AS repeat_state,
    (SELECT r.frequency_anchor_date FROM action_items r WHERE r.id = ti.series_id) AS frequency_anchor_date,
    ti.series_id,
    ti.occurrence_date,
    ti.timezone,
    ti.is_exception,
    (ti.deleted_at IS NOT NULL) AS deleted,
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
WHERE ti.task_id = sqlc.arg(task_id)::text
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'read')
ORDER BY ti.position ASC, ti.occurrence_date ASC;

-- name: ListActionItemsForOccurrenceCommandByTaskAndUserID :many
SELECT
    ti.id,
    ti.task_id,
    ti.title,
    ti.description,
    ti.due_date,
    ti.completed,
    ti.position,
    COALESCE((SELECT r.interval_weeks FROM action_items r WHERE r.id = ti.series_id), 0)::integer AS interval_weeks,
    (SELECT r.repeat_state FROM action_items r WHERE r.id = ti.series_id) AS repeat_state,
    (SELECT r.frequency_anchor_date FROM action_items r WHERE r.id = ti.series_id) AS frequency_anchor_date,
    ti.series_id,
    ti.occurrence_date,
    ti.timezone,
    ti.is_exception,
    (ti.deleted_at IS NOT NULL) AS deleted,
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
WHERE ti.task_id = sqlc.arg(task_id)::text
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(actor_id)::text, sqlc.arg(resource_id)::text, sqlc.arg(action)::action)
ORDER BY ti.position ASC, ti.occurrence_date ASC;

-- name: ListActionItemsByTaskAndUserIDCursorPage :many
SELECT
    ti.id,
    ti.task_id,
    ti.title,
    ti.description,
    ti.due_date,
    ti.completed,
    ti.position,
    COALESCE((SELECT r.interval_weeks FROM action_items r WHERE r.id = ti.series_id ), 0)::integer AS interval_weeks,
    (SELECT r.repeat_state FROM action_items r WHERE r.id = ti.series_id) AS repeat_state,
    (SELECT r.frequency_anchor_date FROM action_items r WHERE r.id = ti.series_id) AS frequency_anchor_date,
    ti.series_id,
    ti.occurrence_date,
    ti.timezone,
    ti.is_exception,
    (ti.deleted_at IS NOT NULL) AS deleted,
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
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, $2, 'action_item', 'read')
  AND ti.deleted_at IS NULL
  AND (sqlc.narg(cursor_position)::integer IS NULL OR (ti.position, ti.occurrence_date, ti.id) > (sqlc.narg(cursor_position)::integer, sqlc.narg(cursor_date)::date, sqlc.narg(cursor_id)::text))
ORDER BY ti.position ASC, ti.occurrence_date ASC, ti.id ASC
LIMIT sqlc.arg(page_limit)::integer;

-- name: ListActiveActionItemSeriesByUserID :many
SELECT
    ti.id,
    ti.task_id,
    ti.title,
    ti.description,
    ti.due_date,
    ti.completed,
    ti.position,
    COALESCE((SELECT r.interval_weeks FROM action_items r WHERE r.id = ti.series_id ), 0)::integer AS interval_weeks,
    (SELECT r.repeat_state FROM action_items r WHERE r.id = ti.series_id) AS repeat_state,
    (SELECT r.frequency_anchor_date FROM action_items r WHERE r.id = ti.series_id) AS frequency_anchor_date,
    ti.series_id,
    ti.occurrence_date,
    ti.timezone,
    ti.is_exception,
    false AS deleted,
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
WHERE t.user_id = $1
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL AND p.status <> 'done'))
  AND t.status <> 'done'
  AND ti.id = ti.series_id
  AND EXISTS (SELECT 1 FROM action_items r WHERE r.id = ti.series_id AND r.repeat_state = 'active')
  AND ti.deleted_at IS NULL
ORDER BY ti.occurrence_date ASC, ti.position ASC;

-- name: NextActionItemPosition :one
SELECT COALESCE(MAX(ti.position), -1)::integer + 1 AS position
FROM action_items AS ti
JOIN tasks AS t ON t.id = ti.task_id
WHERE ti.task_id = $1
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, $2, 'action_item', 'create')
  AND ti.occurrence_date = sqlc.arg(occurrence_date)::date
  AND ti.deleted_at IS NULL;
