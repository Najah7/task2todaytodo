-- name: ReadTaskProgressSources :many
WITH requested AS (
    SELECT unnest(sqlc.arg(task_ids)::text[]) AS task_id
    UNION
    SELECT t.id AS task_id
    FROM tasks t
    WHERE t.project_id = ANY(sqlc.arg(project_ids)::text[])
      AND t.deleted_at IS NULL
), action_item_counts AS (
    SELECT ai.task_id,
           COUNT(*)::bigint AS total,
           COUNT(*) FILTER (WHERE ai.completed)::bigint AS completed
    FROM action_items ai
    JOIN requested r ON r.task_id = ai.task_id
    JOIN tasks t ON t.id = r.task_id
    JOIN projects p ON p.id = t.project_id AND p.deleted_at IS NULL
    WHERE ai.deleted_at IS NULL
    GROUP BY ai.task_id
)
SELECT t.id AS task_id, t.user_id, t.project_id, t.status, p.status AS project_status,
       COALESCE(tc.total, 0)::bigint AS action_item_total,
       COALESCE(tc.completed, 0)::bigint AS action_item_completed
FROM requested r
JOIN tasks t ON t.id = r.task_id
JOIN projects p ON p.id = t.project_id
LEFT JOIN action_item_counts tc ON tc.task_id = t.id
WHERE t.deleted_at IS NULL;

-- name: ReadProjectScheduleProgressCounts :many
SELECT s.project_id,
       COUNT(*) FILTER (WHERE s.deleted_at IS NULL AND s.skipped_at IS NULL)::bigint AS total,
       COUNT(*) FILTER (WHERE s.deleted_at IS NULL AND s.skipped_at IS NULL AND s.completed)::bigint AS completed
FROM schedules s
WHERE s.project_id = ANY(sqlc.arg(project_ids)::text[])
GROUP BY s.project_id;
