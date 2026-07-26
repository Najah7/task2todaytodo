-- name: GetTodoList :one
SELECT id, user_id, list_date, created_at, updated_at
FROM todo_lists
WHERE id = $1;

-- name: GetTodoListByUserAndDate :one
SELECT id, user_id, list_date, created_at, updated_at
FROM todo_lists
WHERE user_id = $1
  AND list_date = $2;

-- name: ListTodoListsByUser :many
SELECT id, user_id, list_date, created_at, updated_at
FROM todo_lists
WHERE user_id = $1
ORDER BY list_date DESC;

-- name: ListTodoListItems :many
SELECT todo_list_id, todo_item_id, position, created_at
FROM todo_list_items
WHERE todo_list_id = $1
ORDER BY position ASC;

-- name: ListTodoListTaskSchedules :many
SELECT tlts.todo_list_id, tlts.task_schedule_id, tlts.created_at
FROM todo_list_task_schedules AS tlts
JOIN task_schedules AS ts ON ts.id = tlts.task_schedule_id
WHERE tlts.todo_list_id = $1
ORDER BY ts.start_at ASC;
