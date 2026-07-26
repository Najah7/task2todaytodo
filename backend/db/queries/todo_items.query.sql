-- name: GetTodoItem :one
SELECT
    ti.id,
    ti.task_id,
    ti.title,
    ti.description,
    ti.due_date,
    ti.completed,
    ti.position,
    ti.interval_weeks,
    ARRAY(
        SELECT tif.frequency
        FROM todo_item_frequencies AS tif
        WHERE tif.todo_item_id = ti.id
        ORDER BY tif.frequency
    )::text[] AS frequencies,
    ti.created_at,
    ti.updated_at
FROM todo_items AS ti
WHERE ti.id = $1;

-- name: GetTodoItemByTaskAndUser :one
SELECT
    ti.id,
    ti.task_id,
    ti.title,
    ti.description,
    ti.due_date,
    ti.completed,
    ti.position,
    ti.interval_weeks,
    ARRAY(
        SELECT tif.frequency
        FROM todo_item_frequencies AS tif
        WHERE tif.todo_item_id = ti.id
        ORDER BY tif.frequency
    )::text[] AS frequencies,
    ti.created_at,
    ti.updated_at
FROM todo_items AS ti
JOIN tasks AS t ON t.id = ti.task_id
WHERE ti.id = $1
  AND ti.task_id = $2
  AND t.user_id = $3;

-- name: ListTodoItemsByTaskAndUser :many
SELECT
    ti.id,
    ti.task_id,
    ti.title,
    ti.description,
    ti.due_date,
    ti.completed,
    ti.position,
    ti.interval_weeks,
    ARRAY(
        SELECT tif.frequency
        FROM todo_item_frequencies AS tif
        WHERE tif.todo_item_id = ti.id
        ORDER BY tif.frequency
    )::text[] AS frequencies,
    ti.created_at,
    ti.updated_at
FROM todo_items AS ti
JOIN tasks AS t ON t.id = ti.task_id
WHERE ti.task_id = $1
  AND t.user_id = $2
ORDER BY ti.position ASC;

-- name: NextTodoItemPosition :one
SELECT COALESCE(MAX(ti.position), -1)::integer + 1 AS position
FROM todo_items AS ti
JOIN tasks AS t ON t.id = ti.task_id
WHERE ti.task_id = $1
  AND t.user_id = $2;
