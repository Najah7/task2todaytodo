-- name: ListProjectMembers :many
SELECT pm.project_id, pm.user_id, pm.role_id, r.name AS role_name,
       u.first_name, u.last_name, u.email, pm.added_by,
       pm.created_at, pm.updated_at
FROM project_members AS pm
JOIN projects AS p ON p.id = pm.project_id
JOIN roles AS r ON r.role_id = pm.role_id
JOIN users AS u ON u.id = pm.user_id
WHERE pm.project_id = sqlc.arg(project_id)::text
  AND p.deleted_at IS NULL
  AND project_has_permission(p.id, sqlc.arg(actor_id)::text, 'project_member', 'read')
ORDER BY u.last_name, u.first_name, pm.user_id;
