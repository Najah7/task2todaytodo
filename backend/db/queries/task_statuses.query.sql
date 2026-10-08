-- name: ListTaskStatuses :many
SELECT status, label, label_jp, created_at, updated_at
FROM status_master;
