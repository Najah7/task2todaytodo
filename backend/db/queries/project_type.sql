-- name: ListProjectTypes :many
SELECT type, label, label_jp, created_at, updated_at
FROM project_type_master;
