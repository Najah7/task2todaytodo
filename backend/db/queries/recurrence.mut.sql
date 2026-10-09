-- name: SetActionItemRecurrenceByTaskAndUserID :execrows
UPDATE action_items AS root
SET repeat_state = 'active',
    frequency_anchor_date = sqlc.arg(frequency_anchor_date)::date,
    interval_weeks = sqlc.arg(interval_weeks)::integer,
    updated_at = now()
FROM tasks AS t
WHERE root.id = sqlc.arg(series_id)::text
  AND root.series_id = root.id
  AND root.task_id = sqlc.arg(task_id)::text
  AND root.deleted_at IS NULL
  AND t.id = root.task_id
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'update')
  AND sqlc.arg(interval_weeks)::integer > 0;

-- name: ReplaceActionItemFrequenciesByTaskAndUserID :exec
DELETE FROM action_item_frequencies f
USING action_items root, tasks t
WHERE f.action_item_id = sqlc.arg(series_id)::text
  AND root.id = f.action_item_id
  AND root.id = root.series_id
  AND root.task_id = sqlc.arg(task_id)::text
  AND root.deleted_at IS NULL
  AND t.id = root.task_id
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'update');
-- name: CreateActionItemFrequenciesByTaskAndUserID :exec
INSERT INTO action_item_frequencies (action_item_id, frequency)
SELECT root.id, f.frequency
FROM action_items root
JOIN tasks t ON t.id = root.task_id
CROSS JOIN unnest(sqlc.arg(frequencies)::text[]) AS f(frequency)
WHERE root.id = sqlc.arg(series_id)::text
  AND root.id = root.series_id
  AND root.task_id = sqlc.arg(task_id)::text
  AND root.deleted_at IS NULL
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'update')
  AND root.repeat_state = 'active'
ON CONFLICT DO NOTHING;

-- name: StopActionItemRecurrenceByTaskAndUserID :execrows
UPDATE action_items AS root
SET repeat_state = 'stopped', interval_weeks = 0, updated_at = now()
FROM tasks AS t
WHERE root.id = sqlc.arg(series_id)::text
  AND root.id = root.series_id
  AND root.task_id = sqlc.arg(task_id)::text
  AND root.deleted_at IS NULL
  AND t.id = root.task_id
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'update')
  AND root.repeat_state IN ('active', 'stopped');

-- name: ClearActionItemFrequenciesByTaskAndUserID :exec
DELETE FROM action_item_frequencies f
USING action_items root, tasks t
WHERE f.action_item_id = sqlc.arg(series_id)::text
  AND root.id = f.action_item_id
  AND root.id = root.series_id
  AND root.task_id = sqlc.arg(task_id)::text
  AND root.deleted_at IS NULL
  AND t.id = root.task_id
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'update');
