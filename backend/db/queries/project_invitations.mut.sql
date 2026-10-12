-- name: LockProjectForInvitationAcceptance :one
SELECT id FROM projects
WHERE id = sqlc.arg(project_id)::text AND deleted_at IS NULL
FOR UPDATE;

-- name: ConsumeProjectInvitation :execrows
UPDATE project_invitations
SET used_at = now(), updated_at = now()
WHERE token_hash = sqlc.arg(token_hash)::bytea
  AND used_at IS NULL AND revoked_at IS NULL AND expires_at > now();

-- name: AcceptProjectInvitationMember :execrows
INSERT INTO project_members (project_id, user_id, role_id, added_by)
SELECT p.id, sqlc.arg(user_id)::text, i.role_id, i.invited_by
FROM project_invitations i
JOIN projects p ON p.id = i.project_id
WHERE i.project_id = sqlc.arg(project_id)::text
  AND i.role_id = sqlc.arg(role_id)::text
  AND i.invited_by = sqlc.arg(invited_by)::text
  AND p.deleted_at IS NULL
  AND p.user_id <> sqlc.arg(user_id)::text
ON CONFLICT (project_id, user_id) DO NOTHING;

-- name: CreateProjectInvitation :one
INSERT INTO project_invitations (id, project_id, email, role_id, invited_by, token_hash, expires_at)
SELECT sqlc.arg(id)::text, p.id, sqlc.arg(email)::citext, sqlc.arg(role_id)::text,
       sqlc.arg(actor_id)::text, sqlc.arg(token_hash)::bytea, sqlc.arg(expires_at)::timestamptz
FROM projects p
WHERE p.id = sqlc.arg(project_id)::text AND p.deleted_at IS NULL
  AND sqlc.arg(role_id)::text IN ('admin', 'editor', 'viewer')
  AND (p.user_id = sqlc.arg(actor_id)::text OR (
       sqlc.arg(role_id)::text IN ('editor', 'viewer') AND EXISTS (
           SELECT 1 FROM project_members actor_pm WHERE actor_pm.project_id = p.id
             AND actor_pm.user_id = sqlc.arg(actor_id)::text AND actor_pm.role_id = 'admin'
       )))
  AND NOT EXISTS (
      SELECT 1 FROM users u WHERE u.email = sqlc.arg(email)::citext
        AND (u.id = p.user_id OR EXISTS (SELECT 1 FROM project_members pm WHERE pm.project_id = p.id AND pm.user_id = u.id))
  )
  AND NOT EXISTS (
      SELECT 1 FROM project_invitations active WHERE active.project_id = p.id
        AND active.email = sqlc.arg(email)::citext AND active.used_at IS NULL
        AND active.revoked_at IS NULL AND active.expires_at > now()
  )
RETURNING id, project_id, email::text AS email, role_id, invited_by, expires_at, created_at, updated_at;

-- name: ResendProjectInvitation :one
UPDATE project_invitations i
SET token_hash = sqlc.arg(token_hash)::bytea,
    expires_at = sqlc.arg(expires_at)::timestamptz,
    updated_at = now()
FROM projects p
WHERE i.id = sqlc.arg(invitation_id)::text AND i.project_id = p.id
  AND p.id = sqlc.arg(project_id)::text AND p.deleted_at IS NULL
  AND i.used_at IS NULL AND i.revoked_at IS NULL
  AND (p.user_id = sqlc.arg(actor_id)::text OR (
       i.role_id IN ('editor', 'viewer') AND EXISTS (
           SELECT 1 FROM project_members actor_pm WHERE actor_pm.project_id = p.id
             AND actor_pm.user_id = sqlc.arg(actor_id)::text AND actor_pm.role_id = 'admin'
       )))
RETURNING i.id, i.project_id, i.email::text AS email, i.role_id, i.invited_by, i.expires_at, i.created_at, i.updated_at;

-- name: RevokeProjectInvitation :execrows
UPDATE project_invitations i
SET revoked_at = now(), updated_at = now()
FROM projects p
WHERE i.id = sqlc.arg(invitation_id)::text AND i.project_id = p.id
  AND p.id = sqlc.arg(project_id)::text AND p.deleted_at IS NULL
  AND i.used_at IS NULL AND i.revoked_at IS NULL
  AND (p.user_id = sqlc.arg(actor_id)::text OR (
       i.role_id IN ('editor', 'viewer') AND EXISTS (
           SELECT 1 FROM project_members actor_pm WHERE actor_pm.project_id = p.id
             AND actor_pm.user_id = sqlc.arg(actor_id)::text AND actor_pm.role_id = 'admin'
       )));

-- name: RevokeInvitationsForInviterWithoutPermission :execrows
UPDATE project_invitations i
SET revoked_at = now(), updated_at = now()
FROM projects p
WHERE p.id = i.project_id
  AND p.id = sqlc.arg(project_id)::text
  AND i.invited_by = sqlc.arg(inviter_id)::text
  AND i.used_at IS NULL AND i.revoked_at IS NULL AND i.expires_at > now()
  AND p.user_id <> i.invited_by
  AND NOT EXISTS (
      SELECT 1 FROM project_members pm
      WHERE pm.project_id = p.id AND pm.user_id = i.invited_by AND pm.role_id = 'admin'
  );
