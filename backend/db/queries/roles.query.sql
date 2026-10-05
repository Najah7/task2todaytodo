-- name: ListRolesWithPermissions :many
SELECT r.role_id, r.name,
       p.permission_id,
       p.resource_id,
       mr.name AS resource_name,
       p.action::text AS action,
       p.effect::text AS effect,
       p.description
FROM roles AS r
LEFT JOIN role_permissions AS rp ON rp.role_id = r.role_id
LEFT JOIN permissions AS p ON p.permission_id = rp.permission_id
LEFT JOIN managed_resources AS mr ON mr.resource_id = p.resource_id
ORDER BY r.role_id, p.resource_id, p.action, p.effect;

-- name: ListPermissions :many
SELECT p.permission_id, p.resource_id, mr.name AS resource_name,
       p.action::text AS action, p.effect::text AS effect, p.description
FROM permissions AS p
JOIN managed_resources AS mr ON mr.resource_id = p.resource_id
ORDER BY p.resource_id, p.action, p.effect;
