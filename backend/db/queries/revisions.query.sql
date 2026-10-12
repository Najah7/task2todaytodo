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

-- name: ListScheduleOccurrenceRevisionsByActor :many
SELECT history.*
FROM schedule_revisions history
JOIN schedules occurrence ON occurrence.id = history.schedule_id
JOIN schedule_rules rule ON rule.id = occurrence.rule_id AND rule.deleted_at IS NULL
WHERE occurrence.rule_id = sqlc.arg(rule_id)::text
  AND occurrence.occurrence_date = sqlc.arg(occurrence_date)::date
  AND (
      (occurrence.deleted_at IS NOT NULL AND occurrence.user_id = sqlc.arg(actor_id)::text)
      OR (occurrence.deleted_at IS NULL AND schedule_has_permission(occurrence.id, sqlc.arg(actor_id)::text, 'schedule', 'read'))
  )
  AND (sqlc.narg(cursor_revision)::integer IS NULL OR history.revision < sqlc.narg(cursor_revision)::integer)
  AND (occurrence.project_id IS NULL OR EXISTS (
      SELECT 1 FROM projects parent WHERE parent.id = occurrence.project_id AND parent.deleted_at IS NULL
  ))
ORDER BY history.revision DESC
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
