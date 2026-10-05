-- name: ReadTaskProgressSources :many
WITH requested AS (
    SELECT unnest(sqlc.arg(task_ids)::text[]) AS task_id
    UNION
    SELECT t.id AS task_id
    FROM tasks t
    WHERE t.project_id = ANY(sqlc.arg(project_ids)::text[])
      AND t.deleted_at IS NULL
),
todo_occurrences AS (
    SELECT DISTINCT ON (ti.task_id, ti.series_id, ti.occurrence_date)
        ti.task_id, ti.completed, ti.deleted_at, ti.skipped_at
    FROM todo_items AS ti
    JOIN requested AS r ON r.task_id = ti.task_id
    ORDER BY ti.task_id, ti.series_id, ti.occurrence_date, (ti.id <> ti.series_id) DESC
),
todo_counts AS (
    SELECT task_id,
           COUNT(*) FILTER (WHERE deleted_at IS NULL AND skipped_at IS NULL)::bigint AS total,
           COUNT(*) FILTER (WHERE deleted_at IS NULL AND skipped_at IS NULL AND completed)::bigint AS completed
    FROM todo_occurrences GROUP BY task_id
),
schedule_occurrences AS (
    SELECT DISTINCT ON (ts.task_id, ts.series_id, ts.occurrence_date)
        ts.task_id, ts.completed, ts.deleted_at, ts.skipped_at
    FROM task_schedules AS ts
    JOIN requested AS r ON r.task_id = ts.task_id
    ORDER BY ts.task_id, ts.series_id, ts.occurrence_date, (ts.id <> ts.series_id) DESC
),
schedule_counts AS (
    SELECT task_id,
           COUNT(*) FILTER (WHERE deleted_at IS NULL AND skipped_at IS NULL)::bigint AS total,
           COUNT(*) FILTER (WHERE deleted_at IS NULL AND skipped_at IS NULL AND completed)::bigint AS completed
    FROM schedule_occurrences GROUP BY task_id
),
todo_roots AS (
    SELECT root.task_id,
           json_agg(json_build_object(
               'series_id', root.id,
               'occurrence_date', root.occurrence_date,
               'timezone', root.timezone,
               'interval_weeks', root.interval_weeks,
               'frequency_anchor_date', root.frequency_anchor_date,
               'frequencies', ARRAY(
                   SELECT f.frequency FROM todo_item_frequencies AS f
                   WHERE f.todo_item_id = root.id ORDER BY f.frequency
               ),
               'occurrence_saved_today',
                   root.occurrence_date = (sqlc.arg(as_of)::timestamptz AT TIME ZONE root.timezone)::date
                   OR EXISTS (
                       SELECT 1 FROM todo_items child
                       WHERE child.series_id = root.id AND child.id <> root.id
                         AND child.occurrence_date = (sqlc.arg(as_of)::timestamptz AT TIME ZONE root.timezone)::date
                   )
           )) AS roots
    FROM todo_items root
    JOIN requested r ON r.task_id = root.task_id
    WHERE root.id = root.series_id AND root.deleted_at IS NULL AND root.repeat_state = 'active'
    GROUP BY root.task_id
),
schedule_roots AS (
    SELECT root.task_id,
           json_agg(json_build_object(
               'series_id', root.id,
               'occurrence_date', root.occurrence_date,
               'timezone', root.timezone,
               'interval_weeks', root.interval_weeks,
               'frequency_anchor_date', root.frequency_anchor_date,
               'frequencies', ARRAY(
                   SELECT f.frequency FROM task_schedule_frequencies AS f
                   WHERE f.task_schedule_id = root.id ORDER BY f.frequency
               ),
               'start_at', root.start_at,
               'end_at', root.end_at,
               'occurrence_saved_today',
                   root.occurrence_date = (sqlc.arg(as_of)::timestamptz AT TIME ZONE root.timezone)::date
                   OR EXISTS (
                       SELECT 1 FROM task_schedules child
                       WHERE child.series_id = root.id AND child.id <> root.id
                         AND child.occurrence_date = (sqlc.arg(as_of)::timestamptz AT TIME ZONE root.timezone)::date
                   )
           )) AS roots
    FROM task_schedules root
    JOIN requested r ON r.task_id = root.task_id
    WHERE root.id = root.series_id AND root.deleted_at IS NULL AND root.repeat_state = 'active'
    GROUP BY root.task_id
)
SELECT t.id AS task_id, t.project_id, t.status,
       COALESCE(tc.total, 0)::bigint AS todo_total,
       COALESCE(tc.completed, 0)::bigint AS todo_completed,
       COALESCE(sc.total, 0)::bigint AS schedule_total,
       COALESCE(sc.completed, 0)::bigint AS schedule_completed,
       COALESCE(tr.roots, '[]'::json)::json AS todo_item_roots,
       COALESCE(sr.roots, '[]'::json)::json AS task_schedule_roots
FROM requested r
JOIN tasks t ON t.id = r.task_id
LEFT JOIN todo_counts tc ON tc.task_id = t.id
LEFT JOIN schedule_counts sc ON sc.task_id = t.id
LEFT JOIN todo_roots tr ON tr.task_id = t.id
LEFT JOIN schedule_roots sr ON sr.task_id = t.id
WHERE t.deleted_at IS NULL;
