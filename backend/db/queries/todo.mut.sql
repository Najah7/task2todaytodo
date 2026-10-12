-- name: CreateTodo :one
INSERT INTO todos (id, user_id, todo_date, timezone, revision)
VALUES (sqlc.arg(id)::text, sqlc.arg(user_id)::text, sqlc.arg(todo_date)::date,
        sqlc.arg(timezone)::text, 1)
RETURNING id, user_id, todo_date, timezone, revision, created_at, updated_at;

-- name: BumpTodoRevision :one
UPDATE todos
SET revision = revision + 1, updated_at = now()
WHERE id = sqlc.arg(id)::text AND revision = sqlc.arg(expected_revision)::integer
RETURNING revision;

-- name: UpdateTodoTimezoneAndBumpRevision :one
UPDATE todos
SET timezone = sqlc.arg(timezone)::text, revision = revision + 1, updated_at = now()
WHERE id = sqlc.arg(id)::text AND revision = sqlc.arg(expected_revision)::integer
RETURNING revision;

-- name: InsertTodoItem :one
INSERT INTO todo_items (
    id, todo_id, kind, action_item_id, task_id, schedule_rule_id, occurrence_date,
    planned_minutes, scheduled_start, scheduled_end, completed, hidden, position
)
VALUES (
    sqlc.arg(id)::text, sqlc.arg(todo_id)::text, sqlc.arg(kind)::text,
    sqlc.narg(action_item_id)::text, sqlc.narg(task_id)::text, sqlc.narg(schedule_rule_id)::text,
    sqlc.narg(occurrence_date)::date, sqlc.narg(planned_minutes)::integer,
    sqlc.narg(scheduled_start)::timestamptz, sqlc.narg(scheduled_end)::timestamptz,
    sqlc.arg(completed)::boolean, sqlc.arg(hidden)::boolean, sqlc.arg(position)::integer
)
RETURNING id, todo_id, kind, action_item_id, task_id, schedule_rule_id, occurrence_date,
          planned_minutes, scheduled_start, scheduled_end, completed, hidden, position,
          created_at, updated_at;

-- name: DeleteTodoItemsExceptCompletedWork :exec
DELETE FROM todo_items
WHERE todo_id = sqlc.arg(todo_id)::text
  AND NOT (kind IN ('action_item','task') AND completed = true);

-- name: DeleteTodoItemByID :execrows
DELETE FROM todo_items
WHERE id = sqlc.arg(id)::text AND todo_id = sqlc.arg(todo_id)::text;

-- name: UpdateTodoItemCompleted :execrows
UPDATE todo_items
SET completed = sqlc.arg(completed)::boolean, updated_at = now()
WHERE id = sqlc.arg(id)::text AND todo_id = sqlc.arg(todo_id)::text;

-- name: UpdateTodoItemPosition :execrows
UPDATE todo_items
SET position = sqlc.arg(position)::integer, updated_at = now()
WHERE id = sqlc.arg(id)::text AND todo_id = sqlc.arg(todo_id)::text AND completed = true;

-- name: UpdateTodoItemState :execrows
UPDATE todo_items
SET completed = sqlc.arg(completed)::boolean, hidden = sqlc.arg(hidden)::boolean,
    position = sqlc.arg(position)::integer, updated_at = now()
WHERE id = sqlc.arg(id)::text AND todo_id = sqlc.arg(todo_id)::text;

-- name: UpdateTodoScheduleHidden :execrows
UPDATE todo_items
SET hidden = sqlc.arg(hidden)::boolean, updated_at = now()
WHERE id = sqlc.arg(id)::text AND todo_id = sqlc.arg(todo_id)::text
  AND kind = 'schedule' AND completed = false;
