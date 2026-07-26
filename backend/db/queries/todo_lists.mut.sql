-- name: CreateTodoList :one
INSERT INTO todo_lists (id, user_id, list_date)
VALUES ($1, $2, $3)
RETURNING id, user_id, list_date, created_at, updated_at;

-- name: DeleteTodoList :exec
DELETE FROM todo_lists
WHERE id = $1;

-- name: AddTodoListItem :one
INSERT INTO todo_list_items (todo_list_id, todo_item_id, position)
VALUES ($1, $2, $3)
RETURNING todo_list_id, todo_item_id, position, created_at;

-- name: UpdateTodoListItemPosition :one
UPDATE todo_list_items
SET position = $3
WHERE todo_list_id = $1
  AND todo_item_id = $2
RETURNING todo_list_id, todo_item_id, position, created_at;

-- name: RemoveTodoListItem :exec
DELETE FROM todo_list_items
WHERE todo_list_id = $1
  AND todo_item_id = $2;

-- name: AddTodoListTaskSchedule :one
INSERT INTO todo_list_task_schedules (todo_list_id, task_schedule_id)
VALUES ($1, $2)
RETURNING todo_list_id, task_schedule_id, created_at;

-- name: RemoveTodoListTaskSchedule :exec
DELETE FROM todo_list_task_schedules
WHERE todo_list_id = $1
  AND task_schedule_id = $2;
