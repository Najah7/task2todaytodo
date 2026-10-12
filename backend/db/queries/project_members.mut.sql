-- name: LockProjectForMemberChange :one
SELECT p.id
FROM projects AS p
WHERE p.id = sqlc.arg(project_id)::text
  AND p.deleted_at IS NULL
FOR UPDATE;

-- name: HasProjectMemberUpsertPermission :one
SELECT project_has_permission(
    p.id,
    sqlc.arg(actor_id)::text,
    'project_member',
    'update'::action
) AND (
    p.user_id = sqlc.arg(actor_id)::text
    OR (
        EXISTS (SELECT 1 FROM project_members actor_member WHERE actor_member.project_id = p.id AND actor_member.user_id = sqlc.arg(actor_id)::text AND actor_member.role_id = 'admin')
        AND sqlc.arg(role_id)::text IN ('editor', 'viewer')
        AND NOT EXISTS (SELECT 1 FROM project_members target WHERE target.project_id = p.id AND target.user_id = sqlc.arg(member_id)::text AND target.role_id = 'admin')
    )
) AND EXISTS (
    SELECT 1 FROM project_members existing
    WHERE existing.project_id = p.id AND existing.user_id = sqlc.arg(member_id)::text
) AS allowed
FROM projects AS p
WHERE p.id = sqlc.arg(project_id)::text AND p.deleted_at IS NULL;

-- name: UpsertProjectMember :execrows
UPDATE project_members AS pm
SET role_id = sqlc.arg(role_id)::text,
    updated_at = now()
FROM projects AS p
WHERE p.id = sqlc.arg(project_id)::text
  AND p.id = pm.project_id
  AND p.deleted_at IS NULL
  AND pm.user_id = sqlc.arg(member_id)::text
  AND pm.role_id <> 'sysadmin'
  AND sqlc.arg(role_id)::text <> 'sysadmin'
  AND (
      p.user_id = sqlc.arg(actor_id)::text
      OR (
          EXISTS (SELECT 1 FROM project_members actor_member WHERE actor_member.project_id = p.id AND actor_member.user_id = sqlc.arg(actor_id)::text AND actor_member.role_id = 'admin')
          AND sqlc.arg(role_id)::text IN ('editor', 'viewer')
          AND pm.role_id IN ('editor', 'viewer')
      )
  );

-- name: ReassignProjectMemberTasks :execrows
UPDATE tasks AS t
SET assignee_id = t.user_id,
    changed_by = sqlc.arg(actor_id)::text
FROM projects AS p
WHERE t.project_id = p.id
  AND p.id = sqlc.arg(project_id)::text
  AND p.deleted_at IS NULL
  AND t.assignee_id = sqlc.arg(member_id)::text
  AND t.deleted_at IS NULL
  AND project_has_permission(p.id, sqlc.arg(actor_id)::text, 'project_member', 'delete');

-- name: DeleteProjectMember :execrows
DELETE FROM project_members AS pm
USING projects AS p
WHERE p.id = pm.project_id
  AND p.id = sqlc.arg(project_id)::text
  AND pm.user_id = sqlc.arg(member_id)::text
  AND p.deleted_at IS NULL
  AND p.user_id <> pm.user_id
  AND (
      p.user_id = sqlc.arg(actor_id)::text
      OR (
          EXISTS (SELECT 1 FROM project_members actor_member WHERE actor_member.project_id = p.id AND actor_member.user_id = sqlc.arg(actor_id)::text AND actor_member.role_id = 'admin')
          AND pm.role_id IN ('editor', 'viewer')
      )
  );
