-- name: ListActionItemSkippedOccurrencesByTaskAndUserID :many
SELECT child.occurrence_date
FROM action_items child
JOIN action_items root ON root.id = child.series_id AND root.series_id = root.id
JOIN tasks t ON t.id = root.task_id
WHERE child.series_id = sqlc.arg(series_id)::text
  AND root.task_id = sqlc.arg(task_id)::text
  AND child.id <> child.series_id
  AND child.deleted_at IS NULL
  AND child.skipped_at IS NOT NULL
  AND root.deleted_at IS NULL
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'read')
ORDER BY child.occurrence_date;
-- name: ListActionItemSkippedOccurrencesForCapabilityByTaskAndUserID :many
SELECT child.occurrence_date
FROM action_items child
JOIN action_items root ON root.id = child.series_id AND root.series_id = root.id
JOIN tasks t ON t.id = root.task_id
WHERE child.series_id = sqlc.arg(series_id)::text
  AND root.task_id = sqlc.arg(task_id)::text
  AND child.id <> child.series_id
  AND child.deleted_at IS NULL
  AND child.skipped_at IS NOT NULL
  AND root.deleted_at IS NULL
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(actor_id)::text, sqlc.arg(resource_id)::text, sqlc.arg(action)::action)
ORDER BY child.occurrence_date;
