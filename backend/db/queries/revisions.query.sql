-- name: ListProjectRevisionsByActor :many
SELECT pr.*
FROM project_revisions AS pr
JOIN projects AS p ON p.id = pr.id
WHERE pr.id = sqlc.arg(id)::text
  AND (
      (p.deleted_at IS NULL AND project_has_permission(p.id, sqlc.arg(actor_id)::text, 'project', 'read'))
      OR (p.deleted_at IS NOT NULL AND (
          p.user_id = sqlc.arg(actor_id)::text
          OR project_has_permission(p.id, sqlc.arg(actor_id)::text, 'deleted_history', 'read')
      ))
  )
  AND (sqlc.narg(cursor_revision)::integer IS NULL OR pr.revision < sqlc.narg(cursor_revision)::integer)
ORDER BY pr.revision DESC
LIMIT sqlc.arg(page_limit)::integer;

-- name: ListTaskRevisionsByActor :many
SELECT tr.*
FROM task_revisions AS tr
JOIN tasks AS t ON t.id = tr.id
LEFT JOIN projects AS p ON p.id = t.project_id
WHERE tr.id = sqlc.arg(id)::text
  AND (
      (t.deleted_at IS NULL AND (p.id IS NULL OR p.deleted_at IS NULL)
       AND task_has_permission(t.id, sqlc.arg(actor_id)::text, 'task', 'read'))
      OR (t.deleted_at IS NOT NULL OR p.deleted_at IS NOT NULL)
       AND (t.user_id = sqlc.arg(actor_id)::text
            OR (p.id IS NOT NULL AND project_has_permission(p.id, sqlc.arg(actor_id)::text, 'deleted_history', 'read')))
  )
  AND (sqlc.narg(cursor_revision)::integer IS NULL OR tr.revision < sqlc.narg(cursor_revision)::integer)
ORDER BY tr.revision DESC
LIMIT sqlc.arg(page_limit)::integer;
