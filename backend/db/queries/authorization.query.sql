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

-- name: HasGlobalPermission :one
SELECT EXISTS (
    SELECT 1
    FROM system_user_roles AS ur
    JOIN role_permissions AS rp ON rp.role_id = ur.role_id
    JOIN permissions AS p ON p.permission_id = rp.permission_id
    WHERE ur.user_id = sqlc.arg(user_id)::text
      AND p.resource_id = sqlc.arg(resource_id)::text
      AND p.action = sqlc.arg(action)::action
      AND p.effect = 'allow'
      AND NOT EXISTS (
          SELECT 1 FROM role_permissions AS deny_grant
          JOIN permissions AS deny ON deny.permission_id = deny_grant.permission_id
          WHERE deny_grant.role_id = ur.role_id AND deny.resource_id = p.resource_id
            AND deny.action = p.action AND deny.effect = 'deny'
      )
);
