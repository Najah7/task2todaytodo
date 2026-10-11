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
SELECT s.project_id,
       COUNT(*) FILTER (WHERE s.deleted_at IS NULL AND s.skipped_at IS NULL)::bigint AS total,
       COUNT(*) FILTER (WHERE s.deleted_at IS NULL AND s.skipped_at IS NULL AND s.completed)::bigint AS completed
FROM schedules s
WHERE s.project_id = ANY(sqlc.arg(project_ids)::text[])
GROUP BY s.project_id;

-- name: ReadProjectScheduleProgressRoots :many
WITH effective AS (
    SELECT DISTINCT ON (b.id)
           r.project_id, b.id AS blueprint_id, r.recurrence_anchor_date,
           r.local_start, r.local_end,
           r.timezone, r.frequency, r.interval_count, r.weekdays, r.end_date, r.cutoff_after,
           (sqlc.arg(as_of)::timestamptz AT TIME ZONE r.timezone)::date AS occurrence_date
    FROM schedule_blueprints b
    JOIN schedule_blueprint_revisions r ON r.blueprint_id = b.id
    WHERE b.deleted_at IS NULL
      AND b.id IN (
          SELECT history.blueprint_id
          FROM schedule_blueprint_revisions history
          WHERE history.project_id = ANY(sqlc.arg(project_ids)::text[])
      )
      AND r.superseded_at IS NULL
      AND r.effective_from <= (sqlc.arg(as_of)::timestamptz AT TIME ZONE r.timezone)::date
    ORDER BY b.id, r.effective_from DESC, r.revision DESC
)
SELECT root.project_id, root.blueprint_id, root.recurrence_anchor_date::text AS recurrence_anchor_date,
       root.local_start, root.local_end,
       root.timezone, root.frequency, root.interval_count, root.weekdays,
       CAST(COALESCE(root.end_date::text, ''::text) AS varchar) AS end_date,
       root.occurrence_date::text AS occurrence_date,
       CAST(COALESCE(root.cutoff_after::text, ''::text) AS varchar) AS cutoff_after,
       EXISTS (
           SELECT 1 FROM schedules saved
           WHERE saved.blueprint_id = root.blueprint_id
             AND saved.occurrence_date = root.occurrence_date
       ) AS occurrence_saved_today,
       pauses.paused_at AS pause_started_at, pauses.resumed_at AS pause_ended_at
FROM effective root
LEFT JOIN schedule_project_pause_intervals pauses
  ON pauses.blueprint_id = root.blueprint_id AND pauses.project_id = root.project_id
WHERE root.frequency <> 'once'
  AND root.project_id = ANY(sqlc.arg(project_ids)::text[])
  AND (root.end_date IS NULL OR root.end_date >= root.occurrence_date)
  AND (root.cutoff_after IS NULL OR root.cutoff_after >= root.occurrence_date);
