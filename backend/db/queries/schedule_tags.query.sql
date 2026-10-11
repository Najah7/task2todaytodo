-- name: ListScheduleTagsByScheduleAndUserID :many
SELECT t.id, t.user_id, t.name, t.created_at, t.updated_at
FROM schedule_tag_assignments assignment
JOIN tags t ON t.id = assignment.tag_id
JOIN schedule_blueprints blueprint ON blueprint.id = assignment.blueprint_id
WHERE assignment.blueprint_id = sqlc.arg(schedule_id)::text
  AND schedule_blueprint_has_permission(blueprint.id, sqlc.arg(user_id)::text, 'schedule', 'read')
  AND t.user_id = blueprint.user_id
ORDER BY t.name, t.id;

-- name: ListScheduleTagsByScheduleIDsAndUserID :many
SELECT assignment.blueprint_id AS schedule_id, t.id, t.user_id, t.name, t.created_at, t.updated_at
FROM schedule_tag_assignments assignment
JOIN schedule_blueprints blueprint ON blueprint.id = assignment.blueprint_id
JOIN tags t ON t.id = assignment.tag_id AND t.user_id = blueprint.user_id
WHERE assignment.blueprint_id = ANY(sqlc.arg(schedule_ids)::text[])
  AND schedule_blueprint_has_permission(blueprint.id, sqlc.arg(user_id)::text, 'schedule', 'read')
ORDER BY assignment.blueprint_id, t.name, t.id;
