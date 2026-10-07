-- name: GetTodoList :one
SELECT id, user_id, list_date, created_at, updated_at
FROM todo_lists
WHERE id = $1;

-- name: GetTodoListByUserIDAndDate :one
SELECT id, user_id, list_date, created_at, updated_at
FROM todo_lists
WHERE user_id = $1
  AND list_date = $2;

-- name: ListTodoListsByUserID :many
SELECT id, user_id, list_date, created_at, updated_at
FROM todo_lists
WHERE user_id = $1
ORDER BY list_date DESC;

-- name: ListTodoListItems :many
SELECT todo_list_id, todo_item_id, position, created_at
FROM todo_list_items
WHERE todo_list_id = $1
ORDER BY position ASC;

-- name: ListTodoListSchedules :many
SELECT tls.todo_list_id, tls.schedule_id, tls.created_at
FROM todo_list_schedules AS tls
JOIN schedules AS s ON s.id = tls.schedule_id
WHERE tls.todo_list_id = $1
ORDER BY s.start_at ASC;
