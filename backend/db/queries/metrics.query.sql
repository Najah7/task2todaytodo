-- name: GetTodoCompletionMetrics :many
WITH user_day AS (
    SELECT users.timezone,
           (now() AT TIME ZONE users.timezone)::date AS end_date,
           (now() AT TIME ZONE users.timezone)::date - (sqlc.arg(days)::integer - 1) AS start_date
    FROM users
    WHERE users.id = sqlc.arg(user_id)::text
), date_rows AS (
    SELECT day::date AS metric_date, user_day.timezone, user_day.start_date, user_day.end_date
    FROM user_day
    CROSS JOIN LATERAL generate_series(user_day.start_date, user_day.end_date, interval '1 day') AS day
), eligible_items AS (
    SELECT todo.todo_date AS metric_date, item.kind, item.task_id AS source_id, item.completed
    FROM todos todo
    JOIN todo_items item ON item.todo_id = todo.id
    JOIN users actor ON actor.id = todo.user_id
    JOIN tasks task ON item.kind = 'task' AND task.id = item.task_id
    WHERE todo.user_id = sqlc.arg(user_id)::text
      AND todo.todo_date BETWEEN (SELECT start_date FROM user_day) AND (SELECT end_date FROM user_day)
      AND task.deleted_at IS NULL AND task.assignee_id = actor.id
      AND task_has_permission(task.id, actor.id, 'task', 'read')
      AND NOT EXISTS (
          SELECT 1
          FROM todo_items all_day_item
          JOIN schedules all_day_occurrence ON all_day_occurrence.rule_id = all_day_item.schedule_rule_id
            AND all_day_occurrence.occurrence_date = all_day_item.occurrence_date
          WHERE all_day_item.todo_id = todo.id AND all_day_item.kind = 'schedule' AND all_day_item.hidden = false
            AND all_day_occurrence.assignee_id = actor.id
            AND all_day_occurrence.all_day = true AND all_day_occurrence.deleted_at IS NULL
            AND all_day_occurrence.skipped_at IS NULL
            AND schedule_occurrence_has_permission(all_day_occurrence.id, actor.id, 'schedule', 'read')
      )
    UNION ALL
    SELECT todo.todo_date AS metric_date, item.kind, item.action_item_id AS source_id, item.completed
    FROM todos todo
    JOIN todo_items item ON item.todo_id = todo.id
    JOIN users actor ON actor.id = todo.user_id
    JOIN action_items action_item ON item.kind = 'action_item' AND action_item.id = item.action_item_id
    JOIN tasks task ON task.id = action_item.task_id
    WHERE todo.user_id = sqlc.arg(user_id)::text
      AND todo.todo_date BETWEEN (SELECT start_date FROM user_day) AND (SELECT end_date FROM user_day)
      AND action_item.deleted_at IS NULL AND task.deleted_at IS NULL AND task.assignee_id = actor.id
      AND task_has_permission(task.id, actor.id, 'action_item', 'read')
      AND NOT EXISTS (
          SELECT 1
          FROM todo_items all_day_item
          JOIN schedules all_day_occurrence ON all_day_occurrence.rule_id = all_day_item.schedule_rule_id
            AND all_day_occurrence.occurrence_date = all_day_item.occurrence_date
          WHERE all_day_item.todo_id = todo.id AND all_day_item.kind = 'schedule' AND all_day_item.hidden = false
            AND all_day_occurrence.assignee_id = actor.id
            AND all_day_occurrence.all_day = true AND all_day_occurrence.deleted_at IS NULL
            AND all_day_occurrence.skipped_at IS NULL
            AND schedule_occurrence_has_permission(all_day_occurrence.id, actor.id, 'schedule', 'read')
      )
    UNION ALL
    SELECT todo.todo_date AS metric_date, item.kind, item.schedule_rule_id || '/' || item.occurrence_date::text AS source_id,
           occurrence.completed
    FROM todos todo
    JOIN todo_items item ON item.todo_id = todo.id AND item.kind = 'schedule' AND item.hidden = false
    JOIN users actor ON actor.id = todo.user_id
    JOIN schedules occurrence ON occurrence.rule_id = item.schedule_rule_id
      AND occurrence.occurrence_date = item.occurrence_date
    WHERE todo.user_id = sqlc.arg(user_id)::text
      AND todo.todo_date BETWEEN (SELECT start_date FROM user_day) AND (SELECT end_date FROM user_day)
      AND occurrence.assignee_id = actor.id
      AND occurrence.deleted_at IS NULL AND occurrence.skipped_at IS NULL
      AND schedule_occurrence_has_permission(occurrence.id, actor.id, 'schedule', 'read')
), daily AS (
    SELECT metric_date, count(*)::integer AS planned_items,
           count(*) FILTER (WHERE completed)::integer AS completed_items
    FROM (
        SELECT metric_date, kind, source_id, bool_and(completed) AS completed
        FROM eligible_items
        WHERE kind IN ('task', 'action_item')
        GROUP BY metric_date, kind, source_id
        UNION ALL
        SELECT metric_date, kind, source_id, bool_and(completed) AS completed
        FROM eligible_items
        WHERE kind = 'schedule'
        GROUP BY metric_date, kind, source_id
    ) grouped
    GROUP BY metric_date
)
SELECT date_rows.metric_date::text AS metric_date, date_rows.timezone,
       date_rows.start_date::text AS start_date, date_rows.end_date::text AS end_date,
       COALESCE(daily.planned_items, 0)::integer AS planned_items,
       COALESCE(daily.completed_items, 0)::integer AS completed_items
FROM date_rows
LEFT JOIN daily ON daily.metric_date = date_rows.metric_date
ORDER BY date_rows.metric_date;

-- name: GetOverdueWorkMetrics :many
WITH actor AS (
    SELECT id, timezone, (now() AT TIME ZONE timezone)::date AS as_of_date
    FROM users
    WHERE id = sqlc.arg(user_id)::text
), eligible_tasks AS (
    SELECT task.id, task.project_id, task.due_date, task.status
    FROM tasks task
    JOIN actor ON actor.id = task.assignee_id
    LEFT JOIN projects project ON project.id = task.project_id
    WHERE task.deleted_at IS NULL AND task.status <> 'done'
      AND (task.project_id IS NULL OR (project.deleted_at IS NULL AND project.status <> 'done'))
      AND task_has_permission(task.id, actor.id, 'task', 'read')
), task_overdue AS (
    SELECT task.project_id, count(*)::integer AS overdue_count
    FROM eligible_tasks task
    CROSS JOIN actor
    WHERE task.due_date < actor.as_of_date
      AND NOT EXISTS (
          SELECT 1 FROM action_items child
          WHERE child.task_id = task.id AND child.deleted_at IS NULL
      )
    GROUP BY task.project_id
), action_item_overdue AS (
    SELECT task.project_id, count(*)::integer AS overdue_count
    FROM eligible_tasks task
    JOIN action_items child ON child.task_id = task.id AND child.deleted_at IS NULL AND child.completed = false
    CROSS JOIN actor
    WHERE task_has_permission(task.id, actor.id, 'action_item', 'read')
      AND COALESCE(child.due_date, task.due_date) < actor.as_of_date
    GROUP BY task.project_id
), combined AS (
    SELECT project_id, SUM(overdue_count)::integer AS overdue_count
    FROM (
        SELECT project_id, overdue_count FROM task_overdue
        UNION ALL
        SELECT project_id, overdue_count FROM action_item_overdue
    ) counts
    GROUP BY project_id
)
SELECT actor.timezone, actor.as_of_date::text AS as_of_date,
       combined.project_id, project.title AS project_title, combined.overdue_count
FROM actor
LEFT JOIN combined ON true
LEFT JOIN projects project ON project.id = combined.project_id
ORDER BY combined.overdue_count DESC NULLS LAST, project.title NULLS LAST, combined.project_id NULLS FIRST;
