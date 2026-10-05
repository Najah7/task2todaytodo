-- name: LockTaskProjectForAssigneeChange :one
SELECT p.id
FROM tasks AS t
JOIN projects AS p ON p.id = t.project_id
WHERE t.id = sqlc.arg(task_id)::text
  AND t.deleted_at IS NULL
  AND p.deleted_at IS NULL
  AND task_has_permission(t.id, sqlc.arg(actor_id)::text, 'task_assignment', 'update')
FOR UPDATE OF p;

-- name: TaskProjectForAssigneeChange :one
SELECT t.project_id
FROM tasks AS t
WHERE t.id = sqlc.arg(task_id)::text
  AND t.deleted_at IS NULL
  AND task_has_permission(t.id, sqlc.arg(actor_id)::text, 'task_assignment', 'update');

-- name: LockTaskForAssigneeChange :one
SELECT t.id
FROM tasks AS t
WHERE t.id = sqlc.arg(task_id)::text
  AND t.deleted_at IS NULL
  AND task_has_permission(t.id, sqlc.arg(actor_id)::text, 'task_assignment', 'update')
FOR UPDATE;

-- name: IsEligibleTaskAssignee :one
SELECT EXISTS (
    SELECT 1
    FROM tasks AS t
    WHERE t.id = sqlc.arg(task_id)::text
      AND t.deleted_at IS NULL
      AND task_has_permission(t.id, sqlc.arg(actor_id)::text, 'task_assignment', 'update')
      AND (
          sqlc.arg(assignee_id)::text = t.user_id
          OR EXISTS (
              SELECT 1 FROM project_members AS pm
              WHERE pm.project_id = t.project_id
                AND pm.user_id = sqlc.arg(assignee_id)::text
          )
      )
) AS eligible;

-- name: ListEligibleTaskAssignees :many
SELECT u.id, u.first_name, u.last_name, u.email,
       (u.id = t.user_id) AS is_project_owner
FROM tasks AS t
JOIN users AS u ON (
    u.id = t.user_id
    OR EXISTS (
        SELECT 1 FROM project_members AS pm
        WHERE pm.project_id = t.project_id AND pm.user_id = u.id
    )
)
WHERE t.id = sqlc.arg(task_id)::text
  AND t.deleted_at IS NULL
  AND task_has_permission(t.id, sqlc.arg(actor_id)::text, 'task_assignment', 'update')
ORDER BY is_project_owner DESC, u.last_name, u.first_name, u.id;
