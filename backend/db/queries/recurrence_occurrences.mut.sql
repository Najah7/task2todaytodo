-- name: UpsertTodoItemOverrideByTaskAndUserID :one
INSERT INTO todo_items (
    id, task_id, title, description, due_date, completed, position,
    series_id, occurrence_date, timezone, is_exception, repeat_state,
    frequency_anchor_date, interval_weeks, deleted_at, skipped_at
)
SELECT sqlc.arg(id)::text, t.id, sqlc.arg(title)::text, sqlc.narg(description)::text,
       sqlc.narg(due_date)::date, sqlc.arg(completed)::boolean, sqlc.arg(position)::integer,
       root.id, sqlc.arg(occurrence_date)::date, sqlc.arg(timezone)::text, true, NULL, NULL, 0,
       CASE WHEN sqlc.arg(deleted)::boolean THEN now() ELSE NULL END, NULL
FROM tasks t
JOIN todo_items root ON root.id = sqlc.arg(series_id)::text AND root.task_id = t.id
WHERE t.id = sqlc.arg(task_id)::text
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'todo_item', 'update')
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND root.deleted_at IS NULL
ON CONFLICT (series_id, occurrence_date) WHERE id <> series_id DO UPDATE SET
    title = EXCLUDED.title,
    description = EXCLUDED.description,
    due_date = EXCLUDED.due_date,
    completed = EXCLUDED.completed,
    position = EXCLUDED.position,
    timezone = EXCLUDED.timezone,
    is_exception = true,
    deleted_at = EXCLUDED.deleted_at,
    updated_at = now()
WHERE todo_items.deleted_at IS NULL
RETURNING id;

-- name: SkipTodoItemOccurrenceByTaskAndUserID :one
INSERT INTO todo_items (
    id, task_id, title, description, due_date, completed, position,
    series_id, occurrence_date, timezone, is_exception, repeat_state,
    frequency_anchor_date, interval_weeks, deleted_at, skipped_at
)
SELECT sqlc.arg(id)::text, root.task_id, root.title, root.description, root.due_date,
       false, root.position, root.id, sqlc.arg(occurrence_date)::date, root.timezone,
       false, NULL, NULL, 0, NULL, now()
FROM todo_items root
JOIN tasks t ON t.id = root.task_id
WHERE root.id = sqlc.arg(series_id)::text
  AND root.task_id = sqlc.arg(task_id)::text
  AND root.series_id = root.id
  AND root.deleted_at IS NULL
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'occurrence', 'update')
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
ON CONFLICT (series_id, occurrence_date) WHERE id <> series_id
DO UPDATE SET skipped_at = COALESCE(todo_items.skipped_at, now()), updated_at = now()
WHERE todo_items.deleted_at IS NULL
RETURNING id;

-- name: RestoreEditedTodoItemOccurrenceByTaskAndUserID :exec
UPDATE todo_items child
SET skipped_at = NULL, updated_at = now()
FROM todo_items root, tasks t
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

-- name: DeleteSkippedTodoItemOccurrenceByTaskAndUserID :exec
DELETE FROM todo_items child
USING todo_items root, tasks t
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

-- name: UpsertTaskScheduleOverrideByTaskAndUserID :one
INSERT INTO task_schedules (
    id, task_id, title, description, location, start_at, end_at,
    series_id, occurrence_date, timezone, is_exception, completed,
    repeat_state, frequency_anchor_date, interval_weeks, deleted_at, skipped_at
)
SELECT sqlc.arg(id)::text, t.id, sqlc.arg(title)::text, sqlc.narg(description)::text,
       sqlc.narg(location)::text, sqlc.arg(start_at)::timestamptz, sqlc.arg(end_at)::timestamptz,
       root.id, sqlc.arg(occurrence_date)::date, sqlc.arg(timezone)::text, true,
       sqlc.arg(completed)::boolean, NULL, NULL, 0,
       CASE WHEN sqlc.arg(deleted)::boolean THEN now() ELSE NULL END, NULL
FROM tasks t
JOIN task_schedules root ON root.id = sqlc.arg(series_id)::text AND root.task_id = t.id
WHERE t.id = sqlc.arg(task_id)::text
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'task_schedule', 'update')
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND root.deleted_at IS NULL
ON CONFLICT (series_id, occurrence_date) WHERE id <> series_id DO UPDATE SET
    title = EXCLUDED.title,
    description = EXCLUDED.description,
    location = EXCLUDED.location,
    start_at = EXCLUDED.start_at,
    end_at = EXCLUDED.end_at,
    timezone = EXCLUDED.timezone,
    is_exception = true,
    completed = EXCLUDED.completed,
    deleted_at = EXCLUDED.deleted_at,
    updated_at = now()
WHERE task_schedules.deleted_at IS NULL
RETURNING id;

-- name: SkipTaskScheduleOccurrenceByTaskAndUserID :one
INSERT INTO task_schedules (
    id, task_id, title, description, location, start_at, end_at,
    series_id, occurrence_date, timezone, is_exception, completed,
    repeat_state, frequency_anchor_date, interval_weeks, deleted_at, skipped_at
)
SELECT sqlc.arg(id)::text, root.task_id, root.title, root.description, root.location,
       root.start_at, root.end_at, root.id, sqlc.arg(occurrence_date)::date, root.timezone,
       false, false, NULL, NULL, 0, NULL, now()
FROM task_schedules root
JOIN tasks t ON t.id = root.task_id
WHERE root.id = sqlc.arg(series_id)::text
  AND root.task_id = sqlc.arg(task_id)::text
  AND root.series_id = root.id
  AND root.deleted_at IS NULL
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'occurrence', 'update')
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
ON CONFLICT (series_id, occurrence_date) WHERE id <> series_id
DO UPDATE SET skipped_at = COALESCE(task_schedules.skipped_at, now()), updated_at = now()
WHERE task_schedules.deleted_at IS NULL
RETURNING id;

-- name: RestoreEditedTaskScheduleOccurrenceByTaskAndUserID :exec
UPDATE task_schedules child
SET skipped_at = NULL, updated_at = now()
FROM task_schedules root, tasks t
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

-- name: DeleteSkippedTaskScheduleOccurrenceByTaskAndUserID :exec
DELETE FROM task_schedules child
USING task_schedules root, tasks t
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
