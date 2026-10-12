-- name: ListProjectMembers :many
WITH roster AS (
    SELECT p.id AS project_id, p.user_id, 'owner'::text AS role_id, 'Owner'::text AS role_name,
           u.first_name, u.last_name, u.email::text AS email, p.user_id AS added_by,
           p.created_at, p.updated_at, 'owner'::text AS status,
           false AS can_edit_role, false AS can_remove
    FROM projects p JOIN users u ON u.id = p.user_id
    WHERE p.id = sqlc.arg(project_id)::text
    UNION ALL
    SELECT pm.project_id, pm.user_id, pm.role_id, r.name AS role_name,
           u.first_name, u.last_name, u.email::text AS email, pm.added_by,
           pm.created_at, pm.updated_at, 'accepted'::text AS status,
           (p.user_id = sqlc.arg(actor_id)::text OR (pm.role_id IN ('editor', 'viewer') AND EXISTS (
                SELECT 1 FROM project_members actor_pm WHERE actor_pm.project_id = p.id AND actor_pm.user_id = sqlc.arg(actor_id)::text AND actor_pm.role_id = 'admin'
           ))) AS can_edit_role,
           (p.user_id = sqlc.arg(actor_id)::text OR (pm.role_id IN ('editor', 'viewer') AND EXISTS (
                SELECT 1 FROM project_members actor_pm WHERE actor_pm.project_id = p.id AND actor_pm.user_id = sqlc.arg(actor_id)::text AND actor_pm.role_id = 'admin'
           ))) AS can_remove
    FROM project_members AS pm
    JOIN projects AS p ON p.id = pm.project_id
    JOIN roles AS r ON r.role_id = pm.role_id
    JOIN users AS u ON u.id = pm.user_id
    WHERE pm.project_id = sqlc.arg(project_id)::text
)
SELECT * FROM roster
WHERE EXISTS (
    SELECT 1 FROM projects p
    WHERE p.id = sqlc.arg(project_id)::text
      AND p.deleted_at IS NULL
      AND project_has_permission(p.id, sqlc.arg(actor_id)::text, 'project_member', 'read')
)
ORDER BY lower(last_name), lower(first_name), user_id;
