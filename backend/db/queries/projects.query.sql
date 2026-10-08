-- name: GetProject :one
SELECT *
FROM projects
WHERE id = $1;

-- name: GetProjectByUserID :one
SELECT *
FROM projects
WHERE id = $1
  AND deleted_at IS NULL
  AND project_has_permission(id, $2, 'project', 'read');

-- name: GetProjectByUserIDForPermission :one
SELECT p.*
FROM projects AS p
WHERE p.id = sqlc.arg(id)::text
  AND p.deleted_at IS NULL
  AND project_has_permission(p.id, sqlc.arg(actor_id)::text, sqlc.arg(resource_id)::text, sqlc.arg(action)::action);

-- name: LockProjectByUserIDForPermission :one
SELECT p.*
FROM projects AS p
WHERE p.id = sqlc.arg(id)::text
  AND p.deleted_at IS NULL
  AND project_has_permission(p.id, sqlc.arg(user_id)::text, sqlc.arg(resource_id)::text, sqlc.arg(action)::action)
FOR UPDATE;

-- name: LockDeletedProjectByUserIDForPermission :one
SELECT p.*
FROM projects AS p
WHERE p.id = sqlc.arg(id)::text
  AND p.deleted_at IS NOT NULL
  AND project_has_permission(p.id, sqlc.arg(user_id)::text, sqlc.arg(resource_id)::text, sqlc.arg(action)::action)
FOR UPDATE;

-- name: LockActiveProjectTasksForDeletion :many
SELECT id
FROM tasks
WHERE project_id = sqlc.arg(project_id)::text
  AND deleted_at IS NULL
ORDER BY id
FOR UPDATE;

-- name: ListProjectsByUserID :many
SELECT *
FROM projects
WHERE deleted_at IS NULL
  AND project_has_permission(projects.id, sqlc.arg(user_id)::text, 'project', 'read')
ORDER BY created_at DESC, id DESC;

-- name: ListProjectsByUserIDPage :many
SELECT *
FROM projects
WHERE deleted_at IS NULL
  AND project_has_permission(projects.id, sqlc.arg(user_id)::text, 'project', 'read')
  AND (sqlc.narg(cursor_at)::timestamptz IS NULL OR (created_at, id) < (sqlc.narg(cursor_at)::timestamptz, sqlc.narg(cursor_id)::text))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_limit)::integer;

-- name: ListProjectCandidates :many
SELECT p.*, pm.weight AS priority_weight,
       project_has_permission(p.id, sqlc.arg(user_id)::text, 'project', 'update') AS can_update,
       project_has_permission(p.id, sqlc.arg(user_id)::text, 'project', 'delete') AS can_delete
FROM projects AS p
JOIN priority_master AS pm ON pm.priority = p.priority
WHERE (sqlc.arg(is_trash)::boolean AND p.deleted_at IS NOT NULL
       AND project_has_permission(p.id, sqlc.arg(user_id)::text, 'project', 'delete'))
   OR (NOT sqlc.arg(is_trash)::boolean AND p.deleted_at IS NULL
       AND project_has_permission(p.id, sqlc.arg(user_id)::text, 'project', 'read')
       AND (sqlc.narg(status)::text IS NULL OR p.status = sqlc.narg(status)::text))
ORDER BY p.created_at DESC, p.id DESC;

-- name: ListProjectPageBySort :many
SELECT p.*, pm.weight AS priority_weight,
       project_has_permission(p.id, sqlc.arg(user_id)::text, 'project', 'update') AS can_update,
       project_has_permission(p.id, sqlc.arg(user_id)::text, 'project', 'delete') AS can_delete
FROM projects AS p
JOIN priority_master AS pm ON pm.priority = p.priority
WHERE ((sqlc.arg(is_trash)::boolean AND p.deleted_at IS NOT NULL
        AND project_has_permission(p.id, sqlc.arg(user_id)::text, 'project', 'delete'))
    OR (NOT sqlc.arg(is_trash)::boolean AND p.deleted_at IS NULL
        AND project_has_permission(p.id, sqlc.arg(user_id)::text, 'project', 'read')
        AND (sqlc.narg(status)::text IS NULL OR p.status = sqlc.narg(status)::text)))
  AND (
    sqlc.narg(anchor_id)::text IS NULL
    OR (
      sqlc.arg(sort_by)::text = 'created_at'
      AND (
        (sqlc.arg(direction)::text = 'forward' AND (
          (sqlc.arg(sort_order)::text = 'desc' AND (p.created_at, p.id) < (sqlc.narg(anchor_at)::timestamptz, sqlc.narg(anchor_id)::text))
          OR (sqlc.arg(sort_order)::text = 'asc' AND (p.created_at, p.id) > (sqlc.narg(anchor_at)::timestamptz, sqlc.narg(anchor_id)::text))
        ))
        OR (sqlc.arg(direction)::text = 'backward' AND (
          (sqlc.arg(sort_order)::text = 'desc' AND (p.created_at, p.id) > (sqlc.narg(anchor_at)::timestamptz, sqlc.narg(anchor_id)::text))
          OR (sqlc.arg(sort_order)::text = 'asc' AND (p.created_at, p.id) < (sqlc.narg(anchor_at)::timestamptz, sqlc.narg(anchor_id)::text))
        ))
      )
    )
    OR (
      sqlc.arg(sort_by)::text = 'title'
      AND (
        (sqlc.arg(direction)::text = 'forward' AND (
          (sqlc.arg(sort_order)::text = 'asc' AND (lower(p.title) > sqlc.narg(anchor_title)::text OR (lower(p.title) = sqlc.narg(anchor_title)::text AND (pm.weight < sqlc.narg(anchor_priority)::integer OR (pm.weight = sqlc.narg(anchor_priority)::integer AND p.id > sqlc.narg(anchor_id)::text)))))
          OR (sqlc.arg(sort_order)::text = 'desc' AND (lower(p.title) < sqlc.narg(anchor_title)::text OR (lower(p.title) = sqlc.narg(anchor_title)::text AND (pm.weight < sqlc.narg(anchor_priority)::integer OR (pm.weight = sqlc.narg(anchor_priority)::integer AND p.id > sqlc.narg(anchor_id)::text)))))
        ))
        OR (sqlc.arg(direction)::text = 'backward' AND (
          (sqlc.arg(sort_order)::text = 'asc' AND (lower(p.title) < sqlc.narg(anchor_title)::text OR (lower(p.title) = sqlc.narg(anchor_title)::text AND (pm.weight > sqlc.narg(anchor_priority)::integer OR (pm.weight = sqlc.narg(anchor_priority)::integer AND p.id < sqlc.narg(anchor_id)::text)))))
          OR (sqlc.arg(sort_order)::text = 'desc' AND (lower(p.title) > sqlc.narg(anchor_title)::text OR (lower(p.title) = sqlc.narg(anchor_title)::text AND (pm.weight > sqlc.narg(anchor_priority)::integer OR (pm.weight = sqlc.narg(anchor_priority)::integer AND p.id < sqlc.narg(anchor_id)::text)))))
        ))
      )
    )
    OR (
      sqlc.arg(sort_by)::text = 'end_date'
      AND (
        (sqlc.narg(anchor_end_date)::date IS NULL AND (
          (sqlc.arg(direction)::text = 'forward' AND p.end_date IS NULL AND (pm.weight < sqlc.narg(anchor_priority)::integer OR (pm.weight = sqlc.narg(anchor_priority)::integer AND p.id > sqlc.narg(anchor_id)::text)))
          OR (sqlc.arg(direction)::text = 'backward' AND (p.end_date IS NOT NULL OR (p.end_date IS NULL AND (pm.weight > sqlc.narg(anchor_priority)::integer OR (pm.weight = sqlc.narg(anchor_priority)::integer AND p.id < sqlc.narg(anchor_id)::text)))))
        ))
        OR (sqlc.narg(anchor_end_date)::date IS NOT NULL AND (
          (sqlc.arg(direction)::text = 'forward' AND (
            p.end_date IS NULL
            OR (sqlc.arg(sort_order)::text = 'asc' AND (p.end_date > sqlc.narg(anchor_end_date)::date OR (p.end_date = sqlc.narg(anchor_end_date)::date AND (pm.weight < sqlc.narg(anchor_priority)::integer OR (pm.weight = sqlc.narg(anchor_priority)::integer AND p.id > sqlc.narg(anchor_id)::text)))))
            OR (sqlc.arg(sort_order)::text = 'desc' AND (p.end_date < sqlc.narg(anchor_end_date)::date OR (p.end_date = sqlc.narg(anchor_end_date)::date AND (pm.weight < sqlc.narg(anchor_priority)::integer OR (pm.weight = sqlc.narg(anchor_priority)::integer AND p.id > sqlc.narg(anchor_id)::text)))))
          ))
          OR (sqlc.arg(direction)::text = 'backward' AND p.end_date IS NOT NULL AND (
            (sqlc.arg(sort_order)::text = 'asc' AND (p.end_date < sqlc.narg(anchor_end_date)::date OR (p.end_date = sqlc.narg(anchor_end_date)::date AND (pm.weight > sqlc.narg(anchor_priority)::integer OR (pm.weight = sqlc.narg(anchor_priority)::integer AND p.id < sqlc.narg(anchor_id)::text)))))
            OR (sqlc.arg(sort_order)::text = 'desc' AND (p.end_date > sqlc.narg(anchor_end_date)::date OR (p.end_date = sqlc.narg(anchor_end_date)::date AND (pm.weight > sqlc.narg(anchor_priority)::integer OR (pm.weight = sqlc.narg(anchor_priority)::integer AND p.id < sqlc.narg(anchor_id)::text)))))
          ))
        ))
      )
    )
  )
ORDER BY
    CASE WHEN sqlc.arg(sort_by)::text = 'created_at' AND ((sqlc.arg(sort_order)::text = 'asc' AND sqlc.arg(direction)::text = 'forward') OR (sqlc.arg(sort_order)::text = 'desc' AND sqlc.arg(direction)::text = 'backward')) THEN p.created_at END ASC,
    CASE WHEN sqlc.arg(sort_by)::text = 'created_at' AND NOT ((sqlc.arg(sort_order)::text = 'asc' AND sqlc.arg(direction)::text = 'forward') OR (sqlc.arg(sort_order)::text = 'desc' AND sqlc.arg(direction)::text = 'backward')) THEN p.created_at END DESC,
    CASE WHEN sqlc.arg(sort_by)::text = 'title' AND ((sqlc.arg(sort_order)::text = 'asc' AND sqlc.arg(direction)::text = 'forward') OR (sqlc.arg(sort_order)::text = 'desc' AND sqlc.arg(direction)::text = 'backward')) THEN lower(p.title) END ASC,
    CASE WHEN sqlc.arg(sort_by)::text = 'title' AND NOT ((sqlc.arg(sort_order)::text = 'asc' AND sqlc.arg(direction)::text = 'forward') OR (sqlc.arg(sort_order)::text = 'desc' AND sqlc.arg(direction)::text = 'backward')) THEN lower(p.title) END DESC,
    CASE WHEN sqlc.arg(sort_by)::text = 'end_date' AND sqlc.arg(direction)::text = 'forward' THEN (p.end_date IS NULL)::integer END ASC,
    CASE WHEN sqlc.arg(sort_by)::text = 'end_date' AND sqlc.arg(direction)::text = 'backward' THEN (p.end_date IS NULL)::integer END DESC,
    CASE WHEN sqlc.arg(sort_by)::text = 'end_date' AND ((sqlc.arg(sort_order)::text = 'asc' AND sqlc.arg(direction)::text = 'forward') OR (sqlc.arg(sort_order)::text = 'desc' AND sqlc.arg(direction)::text = 'backward')) THEN p.end_date END ASC,
    CASE WHEN sqlc.arg(sort_by)::text = 'end_date' AND NOT ((sqlc.arg(sort_order)::text = 'asc' AND sqlc.arg(direction)::text = 'forward') OR (sqlc.arg(sort_order)::text = 'desc' AND sqlc.arg(direction)::text = 'backward')) THEN p.end_date END DESC,
    CASE WHEN sqlc.arg(sort_by)::text <> 'created_at' AND sqlc.arg(direction)::text = 'backward' THEN pm.weight END ASC,
    CASE WHEN sqlc.arg(sort_by)::text <> 'created_at' AND sqlc.arg(direction)::text = 'forward' THEN pm.weight END DESC,
    CASE WHEN sqlc.arg(sort_by)::text = 'created_at' AND ((sqlc.arg(sort_order)::text = 'asc' AND sqlc.arg(direction)::text = 'forward') OR (sqlc.arg(sort_order)::text = 'desc' AND sqlc.arg(direction)::text = 'backward')) THEN p.id END ASC,
    CASE WHEN sqlc.arg(sort_by)::text = 'created_at' AND NOT ((sqlc.arg(sort_order)::text = 'asc' AND sqlc.arg(direction)::text = 'forward') OR (sqlc.arg(sort_order)::text = 'desc' AND sqlc.arg(direction)::text = 'backward')) THEN p.id END DESC,
    CASE WHEN sqlc.arg(sort_by)::text <> 'created_at' AND sqlc.arg(direction)::text = 'backward' THEN p.id END DESC,
    CASE WHEN sqlc.arg(sort_by)::text <> 'created_at' AND sqlc.arg(direction)::text = 'forward' THEN p.id END ASC
LIMIT sqlc.arg(page_limit)::integer;

-- name: ReadProjectListSummary :one
WITH user_calendar AS (
    SELECT timezone, (sqlc.arg(as_of)::timestamptz AT TIME ZONE timezone)::date AS today
    FROM users WHERE id = sqlc.arg(user_id)::text
), live_readable AS (
    SELECT p.* FROM projects p
    WHERE p.deleted_at IS NULL
      AND project_has_permission(p.id, sqlc.arg(user_id)::text, 'project', 'read')
), trash_deletable AS (
    SELECT p.* FROM projects p
    WHERE p.deleted_at IS NOT NULL
      AND project_has_permission(p.id, sqlc.arg(user_id)::text, 'project', 'delete')
), selected AS (
    SELECT p.* FROM projects p, user_calendar c
    WHERE (sqlc.arg(is_trash)::boolean AND p.deleted_at IS NOT NULL
           AND project_has_permission(p.id, sqlc.arg(user_id)::text, 'project', 'delete'))
       OR (NOT sqlc.arg(is_trash)::boolean AND p.deleted_at IS NULL
           AND project_has_permission(p.id, sqlc.arg(user_id)::text, 'project', 'read')
           AND (sqlc.narg(status)::text IS NULL OR p.status = sqlc.narg(status)::text))
)
SELECT
    (SELECT COUNT(*) FROM selected)::bigint AS total_count,
    (SELECT COUNT(*) FROM selected p CROSS JOIN user_calendar c
      WHERE p.end_date >= c.today AND p.end_date <= c.today + 14)::bigint AS due_soon_count,
    (SELECT COUNT(*) FROM selected p CROSS JOIN user_calendar c
      WHERE p.end_date < c.today)::bigint AS overdue_count,
    (SELECT COUNT(*) FROM live_readable WHERE status = 'in_progress')::bigint AS in_progress_count,
    (SELECT COUNT(*) FROM live_readable WHERE status = 'pending')::bigint AS pending_count,
    (SELECT COUNT(*) FROM live_readable WHERE status = 'done')::bigint AS done_count,
    (SELECT COUNT(*) FROM live_readable WHERE status = 'open')::bigint AS open_count,
    (SELECT COUNT(*) FROM live_readable WHERE status = 'waiting_on_others')::bigint AS waiting_on_others_count,
    (SELECT COUNT(*) FROM trash_deletable)::bigint AS trash_count,
    (SELECT today FROM user_calendar) AS today,
    (SELECT timezone FROM user_calendar) AS timezone;

-- name: ListProjectPriorities :many
SELECT priority, label, label_jp, weight, created_at, updated_at
FROM priority_master
ORDER BY weight DESC, priority;

-- name: ListProjectTasksByUserID :many
SELECT t.*
FROM tasks AS t
WHERE t.project_id = $1
  AND t.deleted_at IS NULL
  AND task_has_permission(t.id, $2, 'task', 'read')
  AND EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL)
ORDER BY t.created_at ASC;
-- name: ReadProjectStatusForLifecycle :one
SELECT status
FROM projects
WHERE id = $1
  AND deleted_at IS NULL;

-- name: LockProjectForLifecycle :one
SELECT id
FROM projects
WHERE id = $1
  AND deleted_at IS NULL
FOR UPDATE;
