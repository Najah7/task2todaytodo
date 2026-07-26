-- name: CreateTodoItem :one
INSERT INTO todo_items (id, task_id, title, description, due_date, completed, position, interval_weeks)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, task_id, title, description, due_date, completed, position, interval_weeks, created_at, updated_at;

-- name: CreateTodoItemByTaskAndUser :one
WITH owned_task AS (
    SELECT tasks.id
    FROM tasks
    WHERE tasks.id = sqlc.arg(task_id)
      AND tasks.user_id = sqlc.arg(user_id)
),
next_position AS (
    SELECT COALESCE(sqlc.narg(position)::integer, COALESCE(MAX(ti.position), -1)::integer + 1) AS position
    FROM owned_task AS ot
    LEFT JOIN todo_items AS ti ON ti.task_id = ot.id
),
inserted AS (
    INSERT INTO todo_items (id, task_id, title, description, due_date, completed, position, interval_weeks)
    SELECT
        sqlc.arg(id),
        owned_task.id,
        sqlc.arg(title),
        sqlc.arg(description),
        sqlc.arg(due_date),
        false,
        next_position.position,
        sqlc.arg(interval_weeks)
    FROM owned_task
    CROSS JOIN next_position
    RETURNING id, task_id, title, description, due_date, completed, position, interval_weeks, created_at, updated_at
),
inserted_frequencies AS (
    INSERT INTO todo_item_frequencies (todo_item_id, frequency)
    SELECT inserted.id, unnest(sqlc.arg(frequencies)::text[])
    FROM inserted
    ON CONFLICT DO NOTHING
)
SELECT
    inserted.id,
    inserted.task_id,
    inserted.title,
    inserted.description,
    inserted.due_date,
    inserted.completed,
    inserted.position,
    inserted.interval_weeks,
    ARRAY(
        SELECT tif.frequency
        FROM todo_item_frequencies AS tif
        WHERE tif.todo_item_id = inserted.id
        ORDER BY tif.frequency
    )::text[] AS frequencies,
    inserted.created_at,
    inserted.updated_at
FROM inserted;

-- name: UpdateTodoItem :one
UPDATE todo_items
SET task_id = $2,
    title = $3,
    description = $4,
    due_date = $5,
    completed = $6,
    position = $7,
    interval_weeks = $8,
    updated_at = now()
WHERE id = $1
RETURNING id, task_id, title, description, due_date, completed, position, interval_weeks, created_at, updated_at;

-- name: SetTodoItemCompletedByTaskAndUser :one
UPDATE todo_items AS ti
SET completed = $4,
    updated_at = now()
FROM tasks AS t
WHERE ti.id = $1
  AND ti.task_id = $2
  AND t.id = ti.task_id
  AND t.user_id = $3
RETURNING ti.id, ti.task_id, ti.title, ti.description, ti.due_date, ti.completed, ti.position, ti.interval_weeks, ti.created_at, ti.updated_at;

-- name: DeleteTodoItem :exec
DELETE FROM todo_items
WHERE id = $1;

-- name: DeleteTodoItemByTaskAndUser :one
DELETE FROM todo_items AS ti
USING tasks AS t
WHERE ti.id = $1
  AND ti.task_id = $2
  AND t.id = ti.task_id
  AND t.user_id = $3
RETURNING ti.id;

-- name: DeleteTodoItemFrequencies :exec
DELETE FROM todo_item_frequencies
WHERE todo_item_id = $1;

-- name: AddTodoItemFrequency :exec
INSERT INTO todo_item_frequencies (todo_item_id, frequency)
VALUES ($1, $2);
