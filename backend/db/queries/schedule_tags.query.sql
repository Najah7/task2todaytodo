-- name: ListScheduleTagsByScheduleAndUserID :many
SELECT t.id, t.user_id, t.name, t.created_at, t.updated_at
FROM schedule_tag_assignments AS sta
JOIN tags AS t ON t.id = sta.tag_id
JOIN schedules AS s ON s.id = sta.schedule_id
JOIN schedules AS requested ON requested.series_id = s.id
WHERE requested.id = sqlc.arg(schedule_id)::text
  AND requested.deleted_at IS NULL
  AND requested.series_id = sta.schedule_id
  AND schedule_has_permission(requested.id, sqlc.arg(user_id)::text, 'schedule', 'read')
  AND t.user_id = requested.user_id
ORDER BY t.name, t.id;

-- name: ListScheduleTagsByScheduleIDsAndUserID :many
WITH requested AS (
    SELECT DISTINCT s.id AS schedule_id, s.series_id, s.user_id
    FROM schedules AS s
    WHERE s.id = ANY(sqlc.arg(schedule_ids)::text[])
      AND s.deleted_at IS NULL
      AND schedule_has_permission(s.id, sqlc.arg(user_id)::text, 'schedule', 'read')
)
SELECT requested.schedule_id, t.id, t.user_id, t.name, t.created_at, t.updated_at
FROM requested
JOIN schedule_tag_assignments AS sta ON sta.schedule_id = requested.series_id
JOIN tags AS t ON t.id = sta.tag_id AND t.user_id = requested.user_id
ORDER BY requested.schedule_id, t.name, t.id;
