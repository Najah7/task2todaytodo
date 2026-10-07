-- name: HasProjectPermission :one
SELECT project_has_permission(
    p.id,
    sqlc.arg(actor_id)::text,
    sqlc.arg(resource_id)::text,
    sqlc.arg(action)::action
) AS allowed
FROM projects AS p
WHERE p.id = sqlc.arg(project_id)::text AND p.deleted_at IS NULL;

-- name: HasTaskPermission :one
SELECT task_has_permission(
    t.id,
    sqlc.arg(actor_id)::text,
    sqlc.arg(resource_id)::text,
    sqlc.arg(action)::action
) AS allowed
FROM tasks AS t
WHERE t.id = sqlc.arg(task_id)::text AND t.deleted_at IS NULL;
