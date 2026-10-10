-- name: UpsertActionItemOverrideByTaskAndUserID :one
INSERT INTO action_items (
    id, task_id, title, description, due_date, estimated_minutes, priority, completed, position,
    series_id, occurrence_date, timezone, is_exception, repeat_state,
    frequency_anchor_date, interval_weeks, deleted_at, skipped_at
)
SELECT sqlc.arg(id)::text, t.id, sqlc.arg(title)::text, sqlc.narg(description)::text,
       sqlc.narg(due_date)::date, sqlc.narg(estimated_minutes)::integer, sqlc.arg(priority)::text,
       sqlc.arg(completed)::boolean, sqlc.arg(position)::integer,
       root.id, sqlc.arg(occurrence_date)::date, sqlc.arg(timezone)::text, true, NULL, NULL, 0,
       CASE WHEN sqlc.arg(deleted)::boolean THEN now() ELSE NULL END, NULL
FROM tasks t
JOIN action_items root ON root.id = sqlc.arg(series_id)::text AND root.task_id = t.id
WHERE t.id = sqlc.arg(task_id)::text
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'update')
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND root.deleted_at IS NULL
ON CONFLICT (series_id, occurrence_date) WHERE id <> series_id DO UPDATE SET
    title = EXCLUDED.title,
    description = EXCLUDED.description,
    due_date = EXCLUDED.due_date,
    estimated_minutes = EXCLUDED.estimated_minutes,
    priority = EXCLUDED.priority,
    completed = EXCLUDED.completed,
    position = EXCLUDED.position,
    timezone = EXCLUDED.timezone,
    is_exception = true,
    deleted_at = EXCLUDED.deleted_at,
    updated_at = now()
WHERE action_items.deleted_at IS NULL
RETURNING id;

-- name: SkipActionItemOccurrenceByTaskAndUserID :one
INSERT INTO action_items (
    id, task_id, title, description, due_date, completed, position,
    series_id, occurrence_date, timezone, is_exception, repeat_state,
    frequency_anchor_date, interval_weeks, deleted_at, skipped_at
)
SELECT sqlc.arg(id)::text, root.task_id, root.title, root.description, root.due_date,
       false, root.position, root.id, sqlc.arg(occurrence_date)::date, root.timezone,
       false, NULL, NULL, 0, NULL, now()
FROM action_items root
JOIN tasks t ON t.id = root.task_id
WHERE root.id = sqlc.arg(series_id)::text
  AND root.task_id = sqlc.arg(task_id)::text
  AND root.series_id = root.id
  AND root.deleted_at IS NULL
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'occurrence', 'update')
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
ON CONFLICT (series_id, occurrence_date) WHERE id <> series_id
DO UPDATE SET skipped_at = COALESCE(action_items.skipped_at, now()), updated_at = now()
WHERE action_items.deleted_at IS NULL
RETURNING id;

-- name: RestoreEditedActionItemOccurrenceByTaskAndUserID :exec
UPDATE action_items child
SET skipped_at = NULL, updated_at = now()
FROM action_items root, tasks t
WHERE child.series_id = root.id
  AND root.task_id = t.id
  AND child.series_id = sqlc.arg(series_id)::text
  AND child.occurrence_date = sqlc.arg(occurrence_date)::date
  AND child.task_id = sqlc.arg(task_id)::text
  AND child.id <> child.series_id
  AND child.is_exception
  AND child.deleted_at IS NULL
  AND child.skipped_at IS NOT NULL
  AND root.deleted_at IS NULL
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'occurrence', 'update')
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL));
-- name: DeleteSkippedActionItemOccurrenceByTaskAndUserID :exec
DELETE FROM action_items child
USING action_items root, tasks t
WHERE child.series_id = root.id
  AND root.task_id = t.id
  AND child.series_id = sqlc.arg(series_id)::text
  AND child.occurrence_date = sqlc.arg(occurrence_date)::date
  AND child.task_id = sqlc.arg(task_id)::text
  AND child.id <> child.series_id
  AND NOT child.is_exception
  AND child.deleted_at IS NULL
  AND child.skipped_at IS NOT NULL
  AND root.deleted_at IS NULL
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'occurrence', 'update')
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL));
