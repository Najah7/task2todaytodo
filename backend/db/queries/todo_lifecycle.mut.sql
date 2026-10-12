-- name: CompleteTodayTodoFramesForActionItem :execrows
WITH target_lists AS (
    SELECT todos.id
    FROM todos
    JOIN users ON users.id = todos.user_id
    WHERE todos.user_id = sqlc.arg(user_id)::text
      AND todos.todo_date = (now() AT TIME ZONE users.timezone)::date
), changed AS (
    UPDATE todo_items
    SET completed = true, updated_at = now()
    WHERE todo_id IN (SELECT id FROM target_lists)
      AND kind = 'action_item'
      AND action_item_id = sqlc.arg(action_item_id)::text
      AND completed = false
    RETURNING todo_id
), lists AS (
    UPDATE todos
    SET revision = revision + 1, updated_at = now()
    WHERE id IN (SELECT DISTINCT todo_id FROM changed)
)
SELECT count(*)::bigint FROM changed;

-- name: ReopenTodoFramesForActionItem :execrows
WITH changed AS (
    UPDATE todo_items
    SET completed = false, updated_at = now()
    WHERE kind = 'action_item'
      AND action_item_id = sqlc.arg(action_item_id)::text
      AND completed = true
    RETURNING todo_id
), lists AS (
    UPDATE todos
    SET revision = revision + 1, updated_at = now()
    WHERE id IN (SELECT DISTINCT todo_id FROM changed)
)
SELECT count(*)::bigint FROM changed;

-- name: CompleteTodayTodoFramesForTask :execrows
WITH target_lists AS (
    SELECT todos.id
    FROM todos
    JOIN users ON users.id=todos.user_id
    WHERE todos.user_id=sqlc.arg(user_id)::text
      AND todos.todo_date=(now() AT TIME ZONE users.timezone)::date
), changed AS (
    UPDATE todo_items SET completed=true, updated_at=now()
    WHERE todo_id IN (SELECT id FROM target_lists)
      AND kind='task' AND task_id=sqlc.arg(task_id)::text AND completed=false
    RETURNING todo_id
), lists AS (
    UPDATE todos SET revision=revision+1, updated_at=now()
    WHERE id IN (SELECT DISTINCT todo_id FROM changed)
)
SELECT count(*)::bigint FROM changed;

-- name: ReopenTodoFramesForTask :execrows
WITH changed AS (
    UPDATE todo_items SET completed=false, updated_at=now()
    WHERE kind='task' AND task_id=sqlc.arg(task_id)::text AND completed=true
    RETURNING todo_id
), lists AS (
    UPDATE todos SET revision=revision+1, updated_at=now()
    WHERE id IN (SELECT DISTINCT todo_id FROM changed)
)
SELECT count(*)::bigint FROM changed;
