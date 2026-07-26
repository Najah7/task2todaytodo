-- name: ListTaskPriorities :many
SELECT priority, label, label_jp, weight, created_at, updated_at
FROM priority_master;
