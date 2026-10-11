-- name: AddScheduleTagToScheduleByUserID :one
WITH owned_pair AS (
    SELECT blueprint.id AS blueprint_id, tag.id AS tag_id, blueprint.user_id
    FROM schedule_blueprints blueprint
    JOIN tags tag ON tag.user_id = blueprint.user_id
    WHERE blueprint.id = sqlc.arg(schedule_id)::text
      AND tag.id = sqlc.arg(tag_id)::text
      AND schedule_blueprint_has_permission(blueprint.id, sqlc.arg(user_id)::text, 'schedule', 'update')
), inserted AS (
    INSERT INTO schedule_tag_assignments(blueprint_id, tag_id)
    SELECT blueprint_id, tag_id FROM owned_pair
    ON CONFLICT (blueprint_id, tag_id) DO NOTHING
    RETURNING 1
)
SELECT EXISTS(SELECT 1 FROM owned_pair) AS owned, EXISTS(SELECT 1 FROM inserted) AS added;

-- name: RemoveScheduleTagFromScheduleByUserID :one
WITH owned_pair AS (
    SELECT blueprint.id AS blueprint_id, tag.id AS tag_id, blueprint.user_id
    FROM schedule_blueprints blueprint
    JOIN tags tag ON tag.user_id = blueprint.user_id
    WHERE blueprint.id = sqlc.arg(schedule_id)::text
      AND tag.id = sqlc.arg(tag_id)::text
      AND schedule_blueprint_has_permission(blueprint.id, sqlc.arg(user_id)::text, 'schedule', 'update')
), deleted AS (
    DELETE FROM schedule_tag_assignments assignment
    USING owned_pair
    WHERE assignment.blueprint_id = owned_pair.blueprint_id AND assignment.tag_id = owned_pair.tag_id
    RETURNING 1
)
SELECT EXISTS(SELECT 1 FROM owned_pair) AS owned, EXISTS(SELECT 1 FROM deleted) AS removed;
