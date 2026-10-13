-- name: GetTodoCurrentDay :one
SELECT (now() AT TIME ZONE users.timezone)::date AS local_date,
       users.timezone,
       users.minimum_gap_minutes
FROM users
WHERE users.id = sqlc.arg(user_id)::text;

-- name: ListTodoPlanningActionItemSources :many
SELECT ai.id,
       ai.task_id,
       t.title AS task_title,
       t.project_id,
       COALESCE(p.title, '') AS project_title,
       ai.title,
       COALESCE(ai.description, '') AS description,
       t.status AS task_status,
       COALESCE(ai.priority, t.priority) AS priority,
       COALESCE(aipm.weight, tpm.weight) AS priority_weight,
       COALESCE(ai.due_date, t.due_date) AS due_at,
       ai.estimated_minutes,
       COALESCE((
           SELECT SUM(prior.planned_minutes)
           FROM todo_items prior
           JOIN todos prior_todo ON prior_todo.id = prior.todo_id
           WHERE prior.kind = 'action_item' AND prior.action_item_id = ai.id
             AND prior.completed = true AND prior_todo.todo_date < sqlc.arg(todo_date)::date
       ), 0)::integer AS completed_before_day_minutes,
       EXISTS (
           SELECT 1 FROM todo_items finished
           WHERE finished.kind = 'action_item' AND finished.action_item_id = ai.id AND finished.completed = true
       ) AS has_completed_frame,
       ai.completed,
       ai.actual_minutes,
       task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'update') AS can_update,
       (task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'update')
        AND t.status <> 'done' AND ai.completed = false
        AND (p.id IS NULL OR p.status <> 'done')) AS can_add_to_today,
       task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'read') AS can_read,
       (task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'create')
        AND t.status <> 'done' AND (p.id IS NULL OR p.status <> 'done') AND NOT EXISTS (
            SELECT 1 FROM todo_items started_task WHERE started_task.kind='task' AND started_task.task_id=t.id AND started_task.completed=true
        )) AS can_add_action_item,
       (task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'update') AND t.status <> 'done' AND ai.completed=false AND NOT EXISTS (
            SELECT 1 FROM todo_items finished WHERE finished.kind='action_item' AND finished.action_item_id=ai.id AND finished.completed=true
        )) AS estimate_editable,
       ai.created_at
FROM action_items ai
JOIN tasks t ON t.id = ai.task_id
LEFT JOIN projects p ON p.id = t.project_id AND p.deleted_at IS NULL
LEFT JOIN priority_master aipm ON aipm.priority = ai.priority
LEFT JOIN priority_master tpm ON tpm.priority = t.priority
WHERE ai.deleted_at IS NULL
  AND t.deleted_at IS NULL
  AND t.assignee_id = sqlc.arg(user_id)::text
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'read')
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'update')
  AND t.status <> 'done'
  AND (
      t.status IN ('open', 'in_progress')
      OR (t.status IN ('pending', 'waiting_on_others') AND EXISTS (
          SELECT 1 FROM todo_items started
          WHERE started.kind = 'action_item' AND started.action_item_id = ai.id AND started.completed = true
      ))
  )
  AND ai.completed = false
  AND (p.id IS NULL OR p.status <> 'done')
  AND (t.project_id IS NULL OR p.id IS NOT NULL)
ORDER BY ai.created_at ASC, ai.id ASC;

-- name: ListTodoActionItemSourcesByIDs :many
SELECT ai.id,
       ai.task_id,
       t.title AS task_title,
       t.project_id,
       COALESCE(p.title, '') AS project_title,
       ai.title,
       COALESCE(ai.description, '') AS description,
       t.status AS task_status,
       COALESCE(ai.priority, t.priority) AS priority,
       COALESCE(aipm.weight, tpm.weight) AS priority_weight,
       COALESCE(ai.due_date, t.due_date) AS due_at,
       ai.estimated_minutes,
       COALESCE((
           SELECT SUM(prior.planned_minutes)
           FROM todo_items prior
           JOIN todos prior_todo ON prior_todo.id = prior.todo_id
           WHERE prior.kind = 'action_item' AND prior.action_item_id = ai.id
             AND prior.completed = true AND prior_todo.todo_date < sqlc.arg(todo_date)::date
       ), 0)::integer AS completed_before_day_minutes,
       EXISTS (
           SELECT 1 FROM todo_items finished
           WHERE finished.kind = 'action_item' AND finished.action_item_id = ai.id AND finished.completed = true
       ) AS has_completed_frame,
       ai.completed,
       ai.actual_minutes,
       task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'update') AS can_update,
       (task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'update')
        AND t.status <> 'done' AND ai.completed = false
        AND (p.id IS NULL OR p.status <> 'done')) AS can_add_to_today,
       task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'read') AS can_read,
       (task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'create')
        AND t.status <> 'done' AND (p.id IS NULL OR p.status <> 'done') AND NOT EXISTS (
            SELECT 1 FROM todo_items started_task WHERE started_task.kind='task' AND started_task.task_id=t.id AND started_task.completed=true
        )) AS can_add_action_item,
       (task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'update') AND t.status <> 'done' AND ai.completed=false AND NOT EXISTS (
            SELECT 1 FROM todo_items finished WHERE finished.kind='action_item' AND finished.action_item_id=ai.id AND finished.completed=true
        )) AS estimate_editable,
       ai.created_at
FROM action_items ai
JOIN tasks t ON t.id = ai.task_id
LEFT JOIN projects p ON p.id = t.project_id AND p.deleted_at IS NULL
LEFT JOIN priority_master aipm ON aipm.priority = ai.priority
LEFT JOIN priority_master tpm ON tpm.priority = t.priority
WHERE ai.id = ANY(sqlc.arg(action_item_ids)::text[])
  AND ai.deleted_at IS NULL
  AND t.deleted_at IS NULL
  AND t.assignee_id = sqlc.arg(user_id)::text
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'read')
  AND (t.project_id IS NULL OR p.id IS NOT NULL)
ORDER BY ai.created_at ASC, ai.id ASC;

-- name: ListTodoActionItemHistory :many
SELECT historical.id, historical.action_item_id, historical.planned_minutes, historical.updated_at AS completed_at
FROM todo_items historical
JOIN todos historical_todo ON historical_todo.id = historical.todo_id
JOIN action_items ai ON ai.id = historical.action_item_id
JOIN tasks t ON t.id = ai.task_id
WHERE historical.kind = 'action_item'
  AND historical.completed = true
  AND historical_todo.todo_date < sqlc.arg(todo_date)::date
  AND t.assignee_id = sqlc.arg(user_id)::text
  AND t.deleted_at IS NULL AND ai.deleted_at IS NULL
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'read')
  AND EXISTS (
      SELECT 1 FROM todo_items today
      JOIN todos today_todo ON today_todo.id = today.todo_id
      WHERE today.kind = 'action_item' AND today.action_item_id = ai.id
        AND today_todo.user_id = sqlc.arg(user_id)::text
        AND today_todo.todo_date = sqlc.arg(todo_date)::date
  )
ORDER BY historical.updated_at, historical.id;

-- name: ListTodoPlanningTaskSources :many
SELECT t.id,
       t.title AS task_title,
       t.project_id,
       COALESCE(p.title, '') AS project_title,
       COALESCE(t.description, '') AS description,
       t.status AS task_status,
       t.priority,
       pm.weight AS priority_weight,
       t.due_date AS due_at,
       t.manual_estimated_minutes AS estimated_minutes,
       COALESCE((
           SELECT SUM(prior.planned_minutes)
           FROM todo_items prior
           JOIN todos prior_todo ON prior_todo.id = prior.todo_id
           WHERE prior.kind = 'task' AND prior.task_id = t.id
             AND prior.completed = true AND prior_todo.todo_date < sqlc.arg(todo_date)::date
       ), 0)::integer AS completed_before_day_minutes,
       EXISTS (
           SELECT 1 FROM todo_items finished
           WHERE finished.kind = 'task' AND finished.task_id = t.id AND finished.completed = true
       ) AS has_completed_frame,
       (t.status = 'done') AS completed,
       t.actual_minutes,
       task_has_permission(t.id, sqlc.arg(user_id)::text, 'task', 'update') AS can_update,
       (task_has_permission(t.id, sqlc.arg(user_id)::text, 'task', 'update')
        AND t.status <> 'done' AND (p.id IS NULL OR p.status <> 'done')) AS can_add_to_today,
       task_has_permission(t.id, sqlc.arg(user_id)::text, 'task', 'read') AS can_read,
       (task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'create') AND t.status <> 'done'
        AND (p.id IS NULL OR p.status <> 'done') AND NOT EXISTS (SELECT 1 FROM todo_items locked WHERE locked.kind='task' AND locked.task_id=t.id AND locked.completed=true)) AS can_add_action_item,
       (task_has_permission(t.id, sqlc.arg(user_id)::text, 'task', 'update') AND t.status <> 'done'
        AND NOT EXISTS (SELECT 1 FROM todo_items locked WHERE locked.kind='task' AND locked.task_id=t.id AND locked.completed=true)) AS estimate_editable,
       t.created_at
FROM tasks t
LEFT JOIN projects p ON p.id = t.project_id AND p.deleted_at IS NULL
JOIN priority_master pm ON pm.priority = t.priority
WHERE t.deleted_at IS NULL
  AND t.assignee_id = sqlc.arg(user_id)::text
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'task', 'read')
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'task', 'update')
  AND t.status <> 'done'
  AND (
      t.status IN ('open', 'in_progress')
      OR (t.status IN ('pending', 'waiting_on_others') AND EXISTS (
          SELECT 1 FROM todo_items started
          WHERE started.kind = 'task' AND started.task_id = t.id AND started.completed = true
      ))
  )
  AND t.manual_estimated_minutes > 0
  AND NOT EXISTS (SELECT 1 FROM action_items child WHERE child.task_id=t.id AND child.deleted_at IS NULL)
  AND (p.id IS NULL OR p.status <> 'done')
  AND (t.project_id IS NULL OR p.id IS NOT NULL)
ORDER BY t.created_at ASC, t.id ASC;

-- name: ListTodoTaskSourcesByIDs :many
SELECT t.id,
       t.title AS task_title,
       t.project_id,
       COALESCE(p.title, '') AS project_title,
       COALESCE(t.description, '') AS description,
       t.status AS task_status,
       t.priority,
       pm.weight AS priority_weight,
       t.due_date AS due_at,
       COALESCE(t.manual_estimated_minutes, 0)::integer AS estimated_minutes,
       COALESCE((
           SELECT SUM(prior.planned_minutes)
           FROM todo_items prior
           JOIN todos prior_todo ON prior_todo.id = prior.todo_id
           WHERE prior.kind = 'task' AND prior.task_id = t.id
             AND prior.completed = true AND prior_todo.todo_date < sqlc.arg(todo_date)::date
       ), 0)::integer AS completed_before_day_minutes,
       EXISTS (
           SELECT 1 FROM todo_items finished
           WHERE finished.kind = 'task' AND finished.task_id = t.id AND finished.completed = true
       ) AS has_completed_frame,
       (t.status = 'done') AS completed,
       t.actual_minutes,
       task_has_permission(t.id, sqlc.arg(user_id)::text, 'task', 'update') AS can_update,
       (task_has_permission(t.id, sqlc.arg(user_id)::text, 'task', 'update') AND t.status <> 'done'
        AND t.manual_estimated_minutes > 0 AND (p.id IS NULL OR p.status <> 'done')
        AND NOT EXISTS (SELECT 1 FROM action_items child WHERE child.task_id=t.id AND child.deleted_at IS NULL)) AS can_add_to_today,
       task_has_permission(t.id, sqlc.arg(user_id)::text, 'task', 'read') AS can_read,
       (task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'create') AND t.status <> 'done'
        AND (p.id IS NULL OR p.status <> 'done') AND NOT EXISTS (SELECT 1 FROM todo_items locked WHERE locked.kind='task' AND locked.task_id=t.id AND locked.completed=true)) AS can_add_action_item,
       (task_has_permission(t.id, sqlc.arg(user_id)::text, 'task', 'update') AND t.status <> 'done'
        AND NOT EXISTS (SELECT 1 FROM todo_items locked WHERE locked.kind='task' AND locked.task_id=t.id AND locked.completed=true)) AS estimate_editable,
       t.created_at
FROM tasks t
LEFT JOIN projects p ON p.id = t.project_id AND p.deleted_at IS NULL
JOIN priority_master pm ON pm.priority = t.priority
WHERE t.id = ANY(sqlc.arg(task_ids)::text[])
  AND t.deleted_at IS NULL
  AND t.assignee_id = sqlc.arg(user_id)::text
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'task', 'read')
  AND (t.project_id IS NULL OR p.id IS NOT NULL)
ORDER BY t.created_at ASC, t.id ASC;

-- name: ListTodoTaskHistory :many
SELECT historical.id, historical.task_id, historical.planned_minutes, historical.updated_at AS completed_at
FROM todo_items historical
JOIN todos historical_todo ON historical_todo.id = historical.todo_id
JOIN tasks t ON t.id = historical.task_id
WHERE historical.kind = 'task'
  AND historical.completed = true
  AND historical_todo.todo_date < sqlc.arg(todo_date)::date
  AND t.assignee_id = sqlc.arg(user_id)::text
  AND t.deleted_at IS NULL
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'task', 'read')
  AND NOT EXISTS (SELECT 1 FROM action_items child WHERE child.task_id=t.id AND child.deleted_at IS NULL)
  AND EXISTS (
      SELECT 1 FROM todo_items today
      JOIN todos today_todo ON today_todo.id = today.todo_id
      WHERE today.kind = 'task' AND today.task_id = t.id
        AND today_todo.user_id = sqlc.arg(user_id)::text
        AND today_todo.todo_date = sqlc.arg(todo_date)::date
  )
ORDER BY historical.updated_at, historical.id;
