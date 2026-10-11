-- name: LockScheduleBlueprintForMemberReassignment :one
SELECT blueprint.id
FROM schedule_blueprints blueprint
WHERE blueprint.id = sqlc.arg(blueprint_id)::text AND blueprint.deleted_at IS NULL
FOR UPDATE;
