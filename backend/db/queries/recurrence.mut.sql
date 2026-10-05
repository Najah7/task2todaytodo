-- name: SetTodoItemRecurrenceByTaskAndUserID :execrows
UPDATE todo_items AS root
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
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'todo_item', 'update')
  AND sqlc.arg(interval_weeks)::integer > 0;

-- name: ReplaceTodoItemFrequenciesByTaskAndUserID :exec
DELETE FROM todo_item_frequencies f
USING todo_items root, tasks t
WHERE f.todo_item_id = sqlc.arg(series_id)::text
  AND root.id = f.todo_item_id
  AND root.id = root.series_id
  AND root.task_id = sqlc.arg(task_id)::text
  AND root.deleted_at IS NULL
  AND t.id = root.task_id
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'todo_item', 'update');

-- name: CreateTodoItemFrequenciesByTaskAndUserID :exec
INSERT INTO todo_item_frequencies (todo_item_id, frequency)
SELECT root.id, f.frequency
FROM todo_items root
JOIN tasks t ON t.id = root.task_id
CROSS JOIN unnest(sqlc.arg(frequencies)::text[]) AS f(frequency)
WHERE root.id = sqlc.arg(series_id)::text
  AND root.id = root.series_id
  AND root.task_id = sqlc.arg(task_id)::text
  AND root.deleted_at IS NULL
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'todo_item', 'update')
  AND root.repeat_state = 'active'
ON CONFLICT DO NOTHING;

-- name: StopTodoItemRecurrenceByTaskAndUserID :execrows
UPDATE todo_items AS root
SET repeat_state = 'stopped', interval_weeks = 0, updated_at = now()
FROM tasks AS t
WHERE root.id = sqlc.arg(series_id)::text
  AND root.id = root.series_id
  AND root.task_id = sqlc.arg(task_id)::text
  AND root.deleted_at IS NULL
  AND t.id = root.task_id
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'todo_item', 'update')
  AND root.repeat_state IN ('active', 'stopped');

-- name: ClearTodoItemFrequenciesByTaskAndUserID :exec
DELETE FROM todo_item_frequencies f
USING todo_items root, tasks t
WHERE f.todo_item_id = sqlc.arg(series_id)::text
  AND root.id = f.todo_item_id
  AND root.id = root.series_id
  AND root.task_id = sqlc.arg(task_id)::text
  AND root.deleted_at IS NULL
  AND t.id = root.task_id
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'todo_item', 'update');

-- name: SetTaskScheduleRecurrenceByTaskAndUserID :execrows
UPDATE task_schedules AS root
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
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'task_schedule', 'update')
  AND sqlc.arg(interval_weeks)::integer > 0;

-- name: ReplaceTaskScheduleFrequenciesByTaskAndUserID :exec
DELETE FROM task_schedule_frequencies f
USING task_schedules root, tasks t
WHERE f.task_schedule_id = sqlc.arg(series_id)::text
  AND root.id = f.task_schedule_id
  AND root.id = root.series_id
  AND root.task_id = sqlc.arg(task_id)::text
  AND root.deleted_at IS NULL
  AND t.id = root.task_id
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'task_schedule', 'update');

-- name: CreateTaskScheduleFrequenciesByTaskAndUserID :exec
INSERT INTO task_schedule_frequencies (task_schedule_id, frequency)
SELECT root.id, f.frequency
FROM task_schedules root
JOIN tasks t ON t.id = root.task_id
CROSS JOIN unnest(sqlc.arg(frequencies)::text[]) AS f(frequency)
WHERE root.id = sqlc.arg(series_id)::text
  AND root.id = root.series_id
  AND root.task_id = sqlc.arg(task_id)::text
  AND root.deleted_at IS NULL
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'task_schedule', 'update')
  AND root.repeat_state = 'active'
ON CONFLICT DO NOTHING;

-- name: StopTaskScheduleRecurrenceByTaskAndUserID :execrows
UPDATE task_schedules AS root
SET repeat_state = 'stopped', interval_weeks = 0, updated_at = now()
FROM tasks AS t
WHERE root.id = sqlc.arg(series_id)::text
  AND root.id = root.series_id
  AND root.task_id = sqlc.arg(task_id)::text
  AND root.deleted_at IS NULL
  AND t.id = root.task_id
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'task_schedule', 'update')
  AND root.repeat_state IN ('active', 'stopped');

-- name: ClearTaskScheduleFrequenciesByTaskAndUserID :exec
DELETE FROM task_schedule_frequencies f
USING task_schedules root, tasks t
WHERE f.task_schedule_id = sqlc.arg(series_id)::text
  AND root.id = f.task_schedule_id
  AND root.id = root.series_id
  AND root.task_id = sqlc.arg(task_id)::text
  AND root.deleted_at IS NULL
  AND t.id = root.task_id
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'task_schedule', 'update');
