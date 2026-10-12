-- name: GetTodoByUserDate :one
SELECT id, user_id, todo_date, timezone, revision, created_at, updated_at
FROM todos
WHERE user_id = sqlc.arg(user_id)::text AND todo_date = sqlc.arg(todo_date)::date;

-- name: LockTodoUser :exec
SELECT id FROM users WHERE id = sqlc.arg(user_id)::text FOR UPDATE;

-- name: ListTodoItemsByTodoID :many
SELECT id, todo_id, kind, action_item_id, task_id, schedule_rule_id, occurrence_date,
       planned_minutes, scheduled_start, scheduled_end, completed, hidden, position,
       created_at, updated_at
FROM todo_items
WHERE todo_id = sqlc.arg(todo_id)::text
ORDER BY position, id;

-- name: HasTodayTodoItemForActionItem :one
SELECT EXISTS (
    SELECT 1
    FROM todos todo
    JOIN users ON users.id = todo.user_id
    JOIN todo_items item ON item.todo_id = todo.id
    WHERE todo.user_id = sqlc.arg(user_id)::text
      AND todo.todo_date = (now() AT TIME ZONE users.timezone)::date
      AND item.kind = 'action_item'
      AND item.action_item_id = sqlc.arg(action_item_id)::text
);

-- name: HasTodayTodoWork :one
SELECT EXISTS (
    SELECT 1 FROM todos todo
    JOIN users ON users.id=todo.user_id
    JOIN todo_items item ON item.todo_id=todo.id
    WHERE todo.user_id=sqlc.arg(user_id)::text
      AND todo.todo_date=(now() AT TIME ZONE users.timezone)::date
      AND item.kind=sqlc.arg(kind)::text
      AND CASE item.kind
          WHEN 'task' THEN item.task_id=sqlc.arg(source_id)::text
          WHEN 'action_item' THEN item.action_item_id=sqlc.arg(source_id)::text
          ELSE false END
);

-- name: HasCompletedTodoWork :one
SELECT EXISTS (
    SELECT 1 FROM todo_items
    WHERE kind=sqlc.arg(kind)::text AND completed=true
      AND CASE kind
          WHEN 'task' THEN task_id=sqlc.arg(source_id)::text
          WHEN 'action_item' THEN action_item_id=sqlc.arg(source_id)::text
          ELSE false END
);
