-- name: GetTodoItem :one
SELECT
    ti.id,
    ti.task_id,
    ti.title,
    ti.description,
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

-- name: CreateTodoItem :one
INSERT INTO todo_items (id, task_id, title, description, completed, position, interval_weeks)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, task_id, title, description, completed, position, interval_weeks, created_at, updated_at;

-- name: UpdateTodoItem :one
UPDATE todo_items
SET task_id = $2,
    title = $3,
    description = $4,
    completed = $5,
    position = $6,
    interval_weeks = $7,
    updated_at = now()
WHERE id = $1
RETURNING id, task_id, title, description, completed, position, interval_weeks, created_at, updated_at;

-- name: DeleteTodoItem :exec
DELETE FROM todo_items
WHERE id = $1;
