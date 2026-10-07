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
    CASE WHEN EXISTS (
        SELECT 1 FROM project_members AS existing
        WHERE existing.project_id = p.id AND existing.user_id = sqlc.arg(member_id)::text
    ) THEN 'update'::action ELSE 'create'::action END
) AS allowed
FROM projects AS p
WHERE p.id = sqlc.arg(project_id)::text AND p.deleted_at IS NULL;

-- name: UpsertProjectMember :execrows
INSERT INTO project_members (project_id, user_id, role_id, added_by)
SELECT p.id, sqlc.arg(member_id)::text, sqlc.arg(role_id)::text, sqlc.arg(actor_id)::text
FROM projects AS p
WHERE p.id = sqlc.arg(project_id)::text
  AND p.deleted_at IS NULL
  AND p.user_id <> sqlc.arg(member_id)::text
  AND project_has_permission(
      p.id,
      sqlc.arg(actor_id)::text,
      'project_member',
      CASE WHEN EXISTS (
          SELECT 1 FROM project_members AS existing
          WHERE existing.project_id = p.id AND existing.user_id = sqlc.arg(member_id)::text
      ) THEN 'update'::action ELSE 'create'::action END
  )
ON CONFLICT (project_id, user_id) DO UPDATE
SET role_id = EXCLUDED.role_id,
    added_by = EXCLUDED.added_by,
    updated_at = now()
WHERE project_has_permission(EXCLUDED.project_id, sqlc.arg(actor_id)::text, 'project_member', 'update');

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
  AND project_has_permission(p.id, sqlc.arg(actor_id)::text, 'project_member', 'delete');
