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
WITH requested AS (
    SELECT s.id, s.project_id, s.series_id, s.occurrence_date,
           s.completed, s.deleted_at, s.skipped_at
    FROM schedules s
    WHERE s.project_id = ANY(sqlc.arg(project_ids)::text[])
), schedule_occurrences AS (
    SELECT DISTINCT ON (s.project_id, s.series_id, s.occurrence_date)
        s.project_id, s.completed, s.deleted_at, s.skipped_at
    FROM requested s
    ORDER BY s.project_id, s.series_id, s.occurrence_date,
             (s.deleted_at IS NOT NULL AND s.skipped_at IS NOT NULL) ASC,
             (s.id <> s.series_id) DESC
)
SELECT project_id,
       COUNT(*) FILTER (WHERE deleted_at IS NULL AND skipped_at IS NULL)::bigint AS total,
       COUNT(*) FILTER (WHERE deleted_at IS NULL AND skipped_at IS NULL AND completed)::bigint AS completed
FROM schedule_occurrences
GROUP BY project_id;

-- name: ReadProjectScheduleProgressRoots :many
SELECT root.project_id,
       root.occurrence_date::text AS occurrence_date,
       root.timezone, root.interval_weeks,
       COALESCE(root.frequency_anchor_date, root.occurrence_date) AS frequency_anchor_date,
       ARRAY(
           SELECT f.frequency FROM schedule_frequencies f
           WHERE f.schedule_id = root.id ORDER BY f.frequency
       )::text[] AS frequencies,
       root.start_at, root.end_at,
       (root.occurrence_date = (sqlc.arg(as_of)::timestamptz AT TIME ZONE root.timezone)::date
        OR EXISTS (
            SELECT 1 FROM schedules child
            WHERE child.series_id = root.id AND child.id <> root.id
              AND (child.deleted_at IS NULL OR child.is_exception)
              AND child.occurrence_date = (sqlc.arg(as_of)::timestamptz AT TIME ZONE root.timezone)::date
        )) AS occurrence_saved_today
FROM schedules root
WHERE root.project_id = ANY(sqlc.arg(project_ids)::text[])
  AND root.id = root.series_id
  AND root.deleted_at IS NULL
  AND root.repeat_state = 'active';
