-- name: ListInboxScheduleBlueprintCreatedAt :many
SELECT id, created_at
FROM schedule_blueprints
WHERE id = ANY(sqlc.arg(blueprint_ids)::text[]);

-- name: ListInboxProjectNames :many
SELECT p.id, p.title
FROM projects p
WHERE p.id = ANY(sqlc.arg(project_ids)::text[])
  AND p.deleted_at IS NULL
  AND project_has_permission(p.id, sqlc.arg(actor_id)::text, 'project', 'read');
