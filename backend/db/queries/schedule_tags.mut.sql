-- name: AddScheduleTagToScheduleByUserID :one
WITH owned_pair AS (
    SELECT s.series_id AS schedule_id, t.id AS tag_id
    FROM schedules AS s
    JOIN tags AS t ON t.user_id = s.user_id
    WHERE s.id = sqlc.arg(schedule_id)::text
      AND s.deleted_at IS NULL
      AND t.id = sqlc.arg(tag_id)::text
      AND schedule_has_permission(s.id, sqlc.arg(user_id)::text, 'schedule', 'update')
), inserted AS (
    INSERT INTO schedule_tag_assignments (schedule_id, tag_id)
    SELECT schedule_id, tag_id FROM owned_pair
    ON CONFLICT (schedule_id, tag_id) DO NOTHING
    RETURNING 1
)
SELECT EXISTS (SELECT 1 FROM owned_pair) AS owned,
       EXISTS (SELECT 1 FROM inserted) AS added;

-- name: RemoveScheduleTagFromScheduleByUserID :one
WITH owned_pair AS (
    SELECT s.series_id AS schedule_id, t.id AS tag_id
    FROM schedules AS s
    JOIN tags AS t ON t.user_id = s.user_id
    WHERE s.id = sqlc.arg(schedule_id)::text
      AND s.deleted_at IS NULL
      AND t.id = sqlc.arg(tag_id)::text
      AND schedule_has_permission(s.id, sqlc.arg(user_id)::text, 'schedule', 'update')
), deleted AS (
    DELETE FROM schedule_tag_assignments AS assignment
    USING owned_pair
    WHERE assignment.schedule_id = owned_pair.schedule_id
      AND assignment.tag_id = owned_pair.tag_id
    RETURNING 1
)
SELECT EXISTS (SELECT 1 FROM owned_pair) AS owned,
       EXISTS (SELECT 1 FROM deleted) AS removed;
