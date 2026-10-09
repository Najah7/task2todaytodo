-- name: SnapshotActionItemRootOccurrenceByTaskAndUserID :exec
INSERT INTO action_items (
    id, task_id, title, description, due_date, completed, position,
    series_id, occurrence_date, timezone, is_exception, repeat_state,
    frequency_anchor_date, interval_weeks, deleted_at
)
SELECT sqlc.arg(id)::text, root.task_id, root.title, root.description, root.due_date,
       root.completed, root.position, root.id, root.occurrence_date, root.timezone,
       true, NULL, NULL, 0, root.deleted_at
FROM action_items root
JOIN tasks t ON t.id = root.task_id
WHERE root.id = sqlc.arg(series_id)::text
  AND root.series_id = root.id
  AND root.task_id = sqlc.arg(task_id)::text
  AND root.deleted_at IS NULL
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'update')
ON CONFLICT (series_id, occurrence_date) WHERE id <> series_id DO NOTHING;

-- name: UpdateActionItemSeriesTemplateByTaskAndUserID :execrows
UPDATE action_items root
SET title = sqlc.arg(title)::text,
    description = sqlc.narg(description)::text,
    due_date = sqlc.narg(due_date)::date,
    position = sqlc.arg(position)::integer,
    updated_at = now()
FROM tasks t
WHERE root.id = sqlc.arg(series_id)::text
  AND root.series_id = root.id
  AND root.task_id = sqlc.arg(task_id)::text
  AND root.deleted_at IS NULL
  AND t.id = root.task_id
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'update');
