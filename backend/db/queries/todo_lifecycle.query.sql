-- name: HasCompletedTodoFrameForActionItem :one
SELECT EXISTS (
    SELECT 1 FROM todo_items
    WHERE kind = 'action_item'
      AND action_item_id = sqlc.arg(action_item_id)::text
      AND completed = true
);

-- name: HasCompletedTodoFrameForTask :one
SELECT EXISTS (
    SELECT 1 FROM todo_items
    WHERE kind='task' AND task_id=sqlc.arg(task_id)::text AND completed=true
);

-- name: HasTodayTodoFrameForTask :one
SELECT EXISTS (
    SELECT 1 FROM todos todo
    JOIN users ON users.id=todo.user_id
    JOIN todo_items item ON item.todo_id=todo.id
    WHERE todo.user_id=sqlc.arg(user_id)::text
      AND todo.todo_date=(now() AT TIME ZONE users.timezone)::date
      AND item.kind='task' AND item.task_id=sqlc.arg(task_id)::text
);

-- name: HasTodayTodoWorkForTask :one
SELECT EXISTS (
    SELECT 1 FROM todos todo
    JOIN users ON users.id=todo.user_id
    JOIN todo_items item ON item.todo_id=todo.id
    LEFT JOIN action_items ai ON ai.id=item.action_item_id
    WHERE todo.user_id=sqlc.arg(user_id)::text
      AND todo.todo_date=(now() AT TIME ZONE users.timezone)::date
      AND ((item.kind='task' AND item.task_id=sqlc.arg(task_id)::text)
        OR (item.kind='action_item' AND ai.task_id=sqlc.arg(task_id)::text))
);
