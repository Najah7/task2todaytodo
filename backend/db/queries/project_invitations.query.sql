-- name: GetProjectInvitationByTokenHashForUpdate :one
SELECT i.id, i.project_id, i.email::text AS email, i.role_id, i.invited_by,
       i.expires_at, i.used_at, i.revoked_at, p.title AS project_title,
       p.user_id AS project_owner_id, p.deleted_at,
       COALESCE(inviter_member.role_id, '') AS inviter_role_id,
       CASE WHEN recipient.id IS NULL THEN 'new'
            WHEN recipient.email_verified_at IS NULL THEN 'unverified'
            ELSE 'verified' END AS recipient_status
FROM project_invitations i
JOIN projects p ON p.id = i.project_id
LEFT JOIN project_members inviter_member
  ON inviter_member.project_id = p.id AND inviter_member.user_id = i.invited_by
LEFT JOIN users recipient ON recipient.email = i.email
WHERE i.token_hash = sqlc.arg(token_hash)::bytea
FOR UPDATE OF i, p;

-- name: GetProjectInvitationByTokenHash :one
SELECT i.id, i.project_id, i.email::text AS email, i.role_id, i.invited_by,
       i.expires_at, i.used_at, i.revoked_at, p.title AS project_title,
       p.user_id AS project_owner_id, p.deleted_at,
       COALESCE(inviter_member.role_id, '') AS inviter_role_id,
       CASE WHEN recipient.id IS NULL THEN 'new'
            WHEN recipient.email_verified_at IS NULL THEN 'unverified'
            ELSE 'verified' END AS recipient_status
FROM project_invitations i
JOIN projects p ON p.id = i.project_id
LEFT JOIN project_members inviter_member
  ON inviter_member.project_id = p.id AND inviter_member.user_id = i.invited_by
LEFT JOIN users recipient ON recipient.email = i.email
WHERE i.token_hash = sqlc.arg(token_hash)::bytea;

-- name: GetProjectInvitationForResend :one
SELECT i.id, i.project_id, i.email::text AS email, i.role_id, i.invited_by,
       i.expires_at, i.used_at, i.revoked_at, p.title AS project_title
FROM project_invitations i
JOIN projects p ON p.id = i.project_id
WHERE i.id = sqlc.arg(invitation_id)::text AND i.project_id = sqlc.arg(project_id)::text
  AND p.deleted_at IS NULL AND i.used_at IS NULL AND i.revoked_at IS NULL
FOR UPDATE OF i, p;

-- name: GetProjectMemberActorRole :one
SELECT CASE WHEN p.user_id = sqlc.arg(actor_id)::text THEN 'owner'::text
            ELSE COALESCE(pm.role_id, '') END AS role_id
FROM projects p
LEFT JOIN project_members pm ON pm.project_id = p.id AND pm.user_id = sqlc.arg(actor_id)::text
WHERE p.id = sqlc.arg(project_id)::text AND p.deleted_at IS NULL
  AND project_has_permission(p.id, sqlc.arg(actor_id)::text, 'project_member', 'read');

-- name: ListProjectRosterPage :many
WITH roster AS (
    SELECT p.id AS project_id, p.user_id, 'owner'::text AS role_id, 'Owner'::text AS role_name,
           u.first_name, u.last_name, u.email::text AS email, p.user_id AS added_by,
           p.created_at, p.updated_at, 'owner'::text AS status, ''::text AS invitation_id,
           NULL::timestamptz AS expires_at, false AS can_edit_role, false AS can_remove,
           p.user_id AS sort_id
    FROM projects p JOIN users u ON u.id = p.user_id
    WHERE p.id = sqlc.arg(project_id)::text
    UNION ALL
    SELECT pm.project_id, pm.user_id, pm.role_id, r.name AS role_name,
           u.first_name, u.last_name, u.email::text AS email, pm.added_by,
           pm.created_at, pm.updated_at, 'accepted'::text AS status, ''::text AS invitation_id,
           NULL::timestamptz AS expires_at,
           (p.user_id = sqlc.arg(actor_id)::text OR (pm.role_id IN ('editor', 'viewer') AND EXISTS (
                SELECT 1 FROM project_members actor_pm WHERE actor_pm.project_id = p.id AND actor_pm.user_id = sqlc.arg(actor_id)::text AND actor_pm.role_id = 'admin'
           ))) AS can_edit_role,
           (p.user_id = sqlc.arg(actor_id)::text OR (pm.role_id IN ('editor', 'viewer') AND EXISTS (
                SELECT 1 FROM project_members actor_pm WHERE actor_pm.project_id = p.id AND actor_pm.user_id = sqlc.arg(actor_id)::text AND actor_pm.role_id = 'admin'
           ))) AS can_remove,
           pm.user_id AS sort_id
    FROM project_members pm JOIN projects p ON p.id = pm.project_id
    JOIN roles r ON r.role_id = pm.role_id JOIN users u ON u.id = pm.user_id
    WHERE pm.project_id = sqlc.arg(project_id)::text
    UNION ALL
    SELECT i.project_id, ''::text AS user_id, i.role_id, r.name AS role_name,
           ''::text AS first_name, ''::text AS last_name, i.email::text AS email, i.invited_by AS added_by,
           i.created_at, i.updated_at,
           CASE WHEN i.expires_at <= now() THEN 'expired'::text ELSE 'pending'::text END AS status,
           i.id AS invitation_id, i.expires_at,
           (p.user_id = sqlc.arg(actor_id)::text OR (i.role_id IN ('editor', 'viewer') AND EXISTS (
                SELECT 1 FROM project_members actor_pm WHERE actor_pm.project_id = p.id AND actor_pm.user_id = sqlc.arg(actor_id)::text AND actor_pm.role_id = 'admin'
           ))) AS can_edit_role,
           (p.user_id = sqlc.arg(actor_id)::text OR (i.role_id IN ('editor', 'viewer') AND EXISTS (
                SELECT 1 FROM project_members actor_pm WHERE actor_pm.project_id = p.id AND actor_pm.user_id = sqlc.arg(actor_id)::text AND actor_pm.role_id = 'admin'
           ))) AS can_remove,
           'invitation:' || i.id AS sort_id
    FROM project_invitations i JOIN projects p ON p.id = i.project_id
    JOIN roles r ON r.role_id = i.role_id
    WHERE i.project_id = sqlc.arg(project_id)::text
      AND i.used_at IS NULL AND i.revoked_at IS NULL
)
SELECT roster.*,
       (SELECT count(*)::bigint FROM roster) AS total_count,
       ((SELECT count(*)::bigint FROM roster WHERE status IN ('owner', 'accepted'))) AS member_count,
       ((SELECT count(*)::bigint FROM roster WHERE status = 'pending')) AS pending_invitation_count
FROM roster
WHERE (sqlc.narg(cursor_at)::timestamptz IS NULL
       OR (sqlc.arg(cursor_direction)::text = 'backward'
           AND (roster.created_at, roster.sort_id) > (sqlc.narg(cursor_at)::timestamptz, sqlc.narg(cursor_id)::text))
       OR (sqlc.arg(cursor_direction)::text <> 'backward'
           AND (roster.created_at, roster.sort_id) < (sqlc.narg(cursor_at)::timestamptz, sqlc.narg(cursor_id)::text)))
  AND EXISTS (
    SELECT 1 FROM projects p WHERE p.id = sqlc.arg(project_id)::text AND p.deleted_at IS NULL
      AND project_has_permission(p.id, sqlc.arg(actor_id)::text, 'project_member', 'read')
  )
ORDER BY CASE WHEN sqlc.arg(cursor_direction)::text = 'backward' THEN roster.created_at END ASC,
         CASE WHEN sqlc.arg(cursor_direction)::text <> 'backward' THEN roster.created_at END DESC,
         CASE WHEN sqlc.arg(cursor_direction)::text = 'backward' THEN roster.sort_id END ASC,
         CASE WHEN sqlc.arg(cursor_direction)::text <> 'backward' THEN roster.sort_id END DESC
LIMIT sqlc.arg(page_limit)::integer;
