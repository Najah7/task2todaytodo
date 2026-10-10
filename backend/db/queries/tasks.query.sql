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

-- name: ListTaskListCandidates :many
WITH user_calendar AS (
    SELECT (sqlc.arg(as_of)::timestamptz AT TIME ZONE u.timezone)::date AS today
    FROM users u WHERE u.id = sqlc.arg(user_id)::text
)
SELECT t.*, COALESCE(p.title, '')::text AS project_name, COALESCE(p.status, '')::text AS project_status,
       task_has_permission(t.id, sqlc.arg(user_id)::text, 'task', 'update') AS can_update,
       CASE WHEN t.due_date IS NULL THEN 0::integer ELSE (t.due_date - c.today)::integer END AS remaining_days
FROM tasks t
LEFT JOIN projects p ON p.id = t.project_id
CROSS JOIN user_calendar c
WHERE t.assignee_id = sqlc.arg(user_id)::text
  AND t.deleted_at IS NULL
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'task', 'read')
  AND (sqlc.narg(project_id)::text IS NULL OR t.project_id = sqlc.narg(project_id)::text)
  AND (sqlc.narg(title)::text IS NULL OR POSITION(LOWER(sqlc.narg(title)::text) IN LOWER(t.title)) > 0)
  AND (
    sqlc.arg(due_filter)::text = 'all'
    OR (sqlc.arg(due_filter)::text = 'overdue' AND t.due_date < c.today)
    OR (sqlc.arg(due_filter)::text = 'today' AND t.due_date = c.today)
    OR (sqlc.arg(due_filter)::text = 'due_soon' AND t.due_date BETWEEN c.today AND c.today + 14)
    OR (sqlc.arg(due_filter)::text = 'no_due' AND t.due_date IS NULL)
  );

-- name: ListTaskListPage :many
WITH user_calendar AS (
    SELECT (sqlc.arg(as_of)::timestamptz AT TIME ZONE u.timezone)::date AS today
    FROM users u WHERE u.id = sqlc.arg(user_id)::text
)
SELECT t.*, COALESCE(p.title, '')::text AS project_name, COALESCE(p.status, '')::text AS project_status,
       task_has_permission(t.id, sqlc.arg(user_id)::text, 'task', 'update') AS can_update,
       CASE WHEN t.due_date IS NULL THEN 0::integer ELSE (t.due_date - c.today)::integer END AS remaining_days
FROM tasks t
LEFT JOIN projects p ON p.id = t.project_id
CROSS JOIN user_calendar c
WHERE t.assignee_id = sqlc.arg(user_id)::text
  AND t.deleted_at IS NULL
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'task', 'read')
  AND (sqlc.narg(status)::text IS NULL OR t.status = sqlc.narg(status)::text)
  AND (sqlc.narg(project_id)::text IS NULL OR t.project_id = sqlc.narg(project_id)::text)
  AND (sqlc.narg(title)::text IS NULL OR POSITION(LOWER(sqlc.narg(title)::text) IN LOWER(t.title)) > 0)
  AND (
    sqlc.arg(due_filter)::text = 'all'
    OR (sqlc.arg(due_filter)::text = 'overdue' AND t.due_date < c.today)
    OR (sqlc.arg(due_filter)::text = 'today' AND t.due_date = c.today)
    OR (sqlc.arg(due_filter)::text = 'due_soon' AND t.due_date BETWEEN c.today AND c.today + 14)
    OR (sqlc.arg(due_filter)::text = 'no_due' AND t.due_date IS NULL)
  )
  AND (
    sqlc.narg(anchor_id)::text IS NULL
    OR (
      sqlc.arg(sort_by)::text = 'created_at'
      AND (
        ((sqlc.arg(sort_order)::text = 'desc' AND sqlc.arg(direction)::text = 'forward') OR (sqlc.arg(sort_order)::text = 'asc' AND sqlc.arg(direction)::text = 'backward'))
          AND (t.created_at, t.id) < (sqlc.narg(anchor_at)::timestamptz, sqlc.narg(anchor_id)::text)
        OR ((sqlc.arg(sort_order)::text = 'asc' AND sqlc.arg(direction)::text = 'forward') OR (sqlc.arg(sort_order)::text = 'desc' AND sqlc.arg(direction)::text = 'backward'))
          AND (t.created_at, t.id) > (sqlc.narg(anchor_at)::timestamptz, sqlc.narg(anchor_id)::text)
      )
    )
    OR (
      sqlc.arg(sort_by)::text = 'title'
      AND (
        ((sqlc.arg(sort_order)::text = 'asc' AND sqlc.arg(direction)::text = 'forward') OR (sqlc.arg(sort_order)::text = 'desc' AND sqlc.arg(direction)::text = 'backward'))
          AND LOWER(t.title) > sqlc.narg(anchor_title)::text
        OR ((sqlc.arg(sort_order)::text = 'desc' AND sqlc.arg(direction)::text = 'forward') OR (sqlc.arg(sort_order)::text = 'asc' AND sqlc.arg(direction)::text = 'backward'))
          AND LOWER(t.title) < sqlc.narg(anchor_title)::text
        OR (LOWER(t.title) = sqlc.narg(anchor_title)::text AND (
          (sqlc.arg(direction)::text = 'forward' AND (t.created_at, t.id) < (sqlc.narg(anchor_at)::timestamptz, sqlc.narg(anchor_id)::text))
          OR (sqlc.arg(direction)::text = 'backward' AND (t.created_at, t.id) > (sqlc.narg(anchor_at)::timestamptz, sqlc.narg(anchor_id)::text))
        ))
      )
    )
    OR (
      sqlc.arg(sort_by)::text = 'due_date'
      AND (
        (sqlc.narg(anchor_due_date)::date IS NULL AND (
          (sqlc.arg(direction)::text = 'forward' AND t.due_date IS NULL AND (t.created_at, t.id) < (sqlc.narg(anchor_at)::timestamptz, sqlc.narg(anchor_id)::text))
          OR (sqlc.arg(direction)::text = 'backward' AND (t.due_date IS NOT NULL OR (t.due_date IS NULL AND (t.created_at, t.id) > (sqlc.narg(anchor_at)::timestamptz, sqlc.narg(anchor_id)::text))))
        ))
        OR (
          sqlc.narg(anchor_due_date)::date IS NOT NULL
          AND (
            (sqlc.arg(direction)::text = 'forward' AND (
              t.due_date IS NULL
              OR (sqlc.arg(sort_order)::text = 'asc' AND t.due_date > sqlc.narg(anchor_due_date)::date)
              OR (sqlc.arg(sort_order)::text = 'desc' AND t.due_date < sqlc.narg(anchor_due_date)::date)
              OR (t.due_date = sqlc.narg(anchor_due_date)::date AND (t.created_at, t.id) < (sqlc.narg(anchor_at)::timestamptz, sqlc.narg(anchor_id)::text))
            ))
            OR (sqlc.arg(direction)::text = 'backward' AND t.due_date IS NOT NULL AND (
              (sqlc.arg(sort_order)::text = 'asc' AND t.due_date < sqlc.narg(anchor_due_date)::date)
              OR (sqlc.arg(sort_order)::text = 'desc' AND t.due_date > sqlc.narg(anchor_due_date)::date)
              OR (t.due_date = sqlc.narg(anchor_due_date)::date AND (t.created_at, t.id) > (sqlc.narg(anchor_at)::timestamptz, sqlc.narg(anchor_id)::text))
            ))
          )
        )
      )
    )
  )
ORDER BY
  CASE WHEN sqlc.arg(sort_by)::text = 'created_at' AND ((sqlc.arg(sort_order)::text = 'asc' AND sqlc.arg(direction)::text = 'forward') OR (sqlc.arg(sort_order)::text = 'desc' AND sqlc.arg(direction)::text = 'backward')) THEN t.created_at END ASC,
  CASE WHEN sqlc.arg(sort_by)::text = 'created_at' AND ((sqlc.arg(sort_order)::text = 'desc' AND sqlc.arg(direction)::text = 'forward') OR (sqlc.arg(sort_order)::text = 'asc' AND sqlc.arg(direction)::text = 'backward')) THEN t.created_at END DESC,
  CASE WHEN sqlc.arg(sort_by)::text = 'title' AND ((sqlc.arg(sort_order)::text = 'asc' AND sqlc.arg(direction)::text = 'forward') OR (sqlc.arg(sort_order)::text = 'desc' AND sqlc.arg(direction)::text = 'backward')) THEN LOWER(t.title) END ASC,
  CASE WHEN sqlc.arg(sort_by)::text = 'title' AND ((sqlc.arg(sort_order)::text = 'desc' AND sqlc.arg(direction)::text = 'forward') OR (sqlc.arg(sort_order)::text = 'asc' AND sqlc.arg(direction)::text = 'backward')) THEN LOWER(t.title) END DESC,
  CASE WHEN sqlc.arg(sort_by)::text = 'due_date' AND sqlc.arg(direction)::text = 'forward' THEN (t.due_date IS NULL)::integer END ASC,
  CASE WHEN sqlc.arg(sort_by)::text = 'due_date' AND sqlc.arg(direction)::text = 'backward' THEN (t.due_date IS NULL)::integer END DESC,
  CASE WHEN sqlc.arg(sort_by)::text = 'due_date' AND ((sqlc.arg(sort_order)::text = 'asc' AND sqlc.arg(direction)::text = 'forward') OR (sqlc.arg(sort_order)::text = 'desc' AND sqlc.arg(direction)::text = 'backward')) THEN t.due_date END ASC,
  CASE WHEN sqlc.arg(sort_by)::text = 'due_date' AND ((sqlc.arg(sort_order)::text = 'desc' AND sqlc.arg(direction)::text = 'forward') OR (sqlc.arg(sort_order)::text = 'asc' AND sqlc.arg(direction)::text = 'backward')) THEN t.due_date END DESC,
  CASE WHEN sqlc.arg(sort_by)::text <> 'created_at' AND sqlc.arg(direction)::text = 'forward' THEN t.created_at END DESC,
  CASE WHEN sqlc.arg(sort_by)::text <> 'created_at' AND sqlc.arg(direction)::text = 'backward' THEN t.created_at END ASC,
  CASE WHEN sqlc.arg(sort_by)::text <> 'created_at' AND sqlc.arg(direction)::text = 'forward' THEN t.id END DESC,
  CASE WHEN sqlc.arg(sort_by)::text <> 'created_at' AND sqlc.arg(direction)::text = 'backward' THEN t.id END ASC,
  CASE WHEN sqlc.arg(sort_by)::text = 'created_at' AND ((sqlc.arg(sort_order)::text = 'asc' AND sqlc.arg(direction)::text = 'forward') OR (sqlc.arg(sort_order)::text = 'desc' AND sqlc.arg(direction)::text = 'backward')) THEN t.id END ASC,
  CASE WHEN sqlc.arg(sort_by)::text = 'created_at' AND ((sqlc.arg(sort_order)::text = 'desc' AND sqlc.arg(direction)::text = 'forward') OR (sqlc.arg(sort_order)::text = 'asc' AND sqlc.arg(direction)::text = 'backward')) THEN t.id END DESC
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
SELECT t.*, p.status AS project_status
FROM tasks AS t
JOIN projects AS p ON p.id = t.project_id AND p.user_id = t.user_id AND p.deleted_at IS NULL
WHERE t.project_id = sqlc.arg(project_id)
  AND t.deleted_at IS NULL
  AND task_has_permission(t.id, sqlc.arg(user_id), 'task', 'read')
  AND (sqlc.narg(cursor_at)::timestamptz IS NULL OR (t.created_at, t.id) < (sqlc.narg(cursor_at)::timestamptz, sqlc.narg(cursor_id)::text))
ORDER BY t.created_at DESC, t.id DESC
LIMIT sqlc.arg(page_limit)::integer;

-- name: ListTasksByProjectAndUserID :many
SELECT t.id, t.user_id, t.project_id, t.title, t.description, t.due_date, t.manual_estimated_minutes, t.actual_minutes,
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
    ti.estimated_minutes,
    ti.priority,
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
