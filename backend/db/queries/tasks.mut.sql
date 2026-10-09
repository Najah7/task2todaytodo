
-- name: CreateTask :one
INSERT INTO tasks (
    id, user_id, project_id, assignee_id, title, description, due_date, estimated_minutes, actual_minutes,
    priority, status, changed_by
)
VALUES ($1, $2, $3, $2, $4, $5, $6, $7, $8, $9, $10, $2)
RETURNING *;

-- name: CreateTaskInProject :one
INSERT INTO tasks (
    id, user_id, project_id, assignee_id, title, description, due_date, estimated_minutes, actual_minutes,
    priority, status, changed_by
)
SELECT
    sqlc.arg(id),
    sqlc.arg(user_id),
    p.id,
    p.user_id,
    sqlc.arg(title),
    sqlc.arg(description),
    sqlc.arg(due_date),
    sqlc.arg(estimated_minutes),
    sqlc.arg(actual_minutes),
    sqlc.arg(priority),
    sqlc.arg(status),
    sqlc.arg(actor_id)
FROM projects AS p
WHERE p.id = sqlc.arg(project_id)
  AND p.deleted_at IS NULL
  AND p.user_id = sqlc.arg(user_id)
  AND project_has_permission(p.id, sqlc.arg(actor_id), 'task', 'create')
RETURNING *;

-- name: UpdateTask :one
UPDATE tasks
SET user_id = $2,
    project_id = $3,
    title = $4,
    description = $5,
    due_date = $6,
    estimated_minutes = $7,
    actual_minutes = $8,
    priority = $9,
    status = $10,
    changed_by = user_id
WHERE id = $1
RETURNING *;

-- name: UpdateTaskByUserID :one
UPDATE tasks
SET title = sqlc.arg(title),
    description = sqlc.arg(description),
    due_date = sqlc.arg(due_date),
    estimated_minutes = sqlc.arg(estimated_minutes),
    actual_minutes = sqlc.arg(actual_minutes),
    changed_by = sqlc.arg(user_id)::text
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
  AND revision = sqlc.arg(expected_revision)::integer
  AND task_has_permission(id, sqlc.arg(user_id)::text, 'task', 'update')
RETURNING *;

-- name: DeleteTask :exec
UPDATE tasks
SET deleted_at = now(), changed_by = user_id
WHERE id = $1 AND deleted_at IS NULL;

-- name: LockTaskByUserID :one
SELECT * FROM tasks AS t
WHERE t.id = sqlc.arg(id)
  AND task_has_permission(t.id, sqlc.arg(user_id), 'task', 'update')
FOR UPDATE;

-- name: LockTaskByUserIDForPermission :one
SELECT * FROM tasks AS t
WHERE t.id = sqlc.arg(id)
  AND t.deleted_at IS NULL
  AND task_has_permission(t.id, sqlc.arg(user_id), sqlc.arg(resource_id)::text, sqlc.arg(action)::action)
FOR UPDATE;

-- name: UpdateTaskStatusByUserID :one
UPDATE tasks
SET status = sqlc.arg(status), changed_by = sqlc.arg(user_id)
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
  AND revision = sqlc.arg(expected_revision)::integer
  AND task_has_permission(id, sqlc.arg(user_id), sqlc.arg(resource_id)::text, sqlc.arg(action)::action)
RETURNING *;

-- name: UpdateTaskStatusDerivedByUserID :execrows
UPDATE tasks
SET status = sqlc.arg(status), changed_by = sqlc.arg(user_id)
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
  AND status IS DISTINCT FROM sqlc.arg(status)::text
  AND task_has_permission(id, sqlc.arg(user_id), sqlc.arg(resource_id)::text, sqlc.arg(action)::action);

-- name: DeleteTaskByUserID :one
WITH deleted_task AS (
    UPDATE tasks
    SET deleted_at = now(), changed_by = sqlc.arg(user_id)
    WHERE tasks.id = sqlc.arg(id)
      AND deleted_at IS NULL
      AND revision = sqlc.arg(expected_revision)::integer
      AND task_has_permission(id, sqlc.arg(user_id), 'task', 'delete')
    RETURNING id
), deleted_items AS (
    UPDATE action_items SET deleted_at = now(), updated_at = now()
    WHERE task_id IN (SELECT id FROM deleted_task) AND deleted_at IS NULL
    RETURNING id
)
SELECT id FROM deleted_task;

-- name: DeleteProjectTasksByActor :execrows
UPDATE tasks AS t
SET deleted_at = now(), changed_by = sqlc.arg(actor_id)::text
WHERE t.project_id = sqlc.arg(project_id)::text
  AND t.deleted_at IS NULL;

-- name: DeleteProjectActionItemsByActor :execrows
UPDATE action_items AS i
SET deleted_at = now(), updated_at = now()
WHERE i.task_id IN (
    SELECT t.id FROM tasks AS t
    WHERE t.project_id = sqlc.arg(project_id)::text
)
  AND i.deleted_at IS NULL;

-- name: AssignTaskToProjectByUserID :one
UPDATE tasks AS t
SET project_id = p.id,
    assignee_id = CASE WHEN t.assignee_id = t.user_id OR EXISTS (
        SELECT 1 FROM project_members pm WHERE pm.project_id = p.id AND pm.user_id = t.assignee_id
    ) THEN t.assignee_id ELSE t.user_id END,
    changed_by = sqlc.arg(user_id)
FROM projects AS p
WHERE t.id = sqlc.arg(task_id)
  AND t.deleted_at IS NULL
  AND t.revision = sqlc.arg(expected_revision)::integer
  AND task_has_permission(t.id, sqlc.arg(user_id), 'task', 'update')
  AND p.id = sqlc.arg(project_id)
  AND p.deleted_at IS NULL
  AND p.user_id = t.user_id
  AND project_has_permission(p.id, sqlc.arg(user_id), 'task', 'create')
RETURNING t.*;

-- name: RemoveTaskFromProjectByUserID :one
UPDATE tasks
SET project_id = NULL,
    assignee_id = user_id,
    changed_by = sqlc.arg(user_id)
WHERE id = sqlc.arg(task_id)
  AND deleted_at IS NULL
  AND revision = sqlc.arg(expected_revision)::integer
  AND project_id = sqlc.arg(project_id)
  AND task_has_permission(id, sqlc.arg(user_id), 'task', 'update')
  AND project_has_permission(project_id, sqlc.arg(user_id), 'task', 'update')
RETURNING *;

-- name: UpdateTaskAssigneeByActor :one
UPDATE tasks AS t
SET assignee_id = sqlc.arg(assignee_id)::text,
    changed_by = sqlc.arg(actor_id)::text
WHERE t.id = sqlc.arg(task_id)::text
  AND t.deleted_at IS NULL
  AND t.revision = sqlc.arg(expected_revision)::integer
  AND task_has_permission(t.id, sqlc.arg(actor_id)::text, 'task_assignment', 'update')
  AND (
      sqlc.arg(assignee_id)::text = t.user_id
      OR EXISTS (
          SELECT 1 FROM project_members AS pm
          WHERE pm.project_id = t.project_id AND pm.user_id = sqlc.arg(assignee_id)::text
      )
  )
RETURNING *;
