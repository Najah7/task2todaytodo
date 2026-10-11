-- name: CreateActionItemByTaskAndUserID :one
WITH owned_task AS (
    SELECT t.id
    FROM tasks AS t
    WHERE t.id = sqlc.arg(task_id)::text
      AND t.deleted_at IS NULL
      AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
      AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'create')
      AND t.status <> 'done'
), next_position AS (
    SELECT COALESCE(sqlc.narg(position)::integer, COALESCE(MAX(ai.position), -1)::integer + 1) AS position
    FROM owned_task AS t
    LEFT JOIN action_items AS ai ON ai.task_id = t.id AND ai.deleted_at IS NULL
), inserted AS (
    INSERT INTO action_items (id, task_id, title, description, due_date, estimated_minutes, priority, completed, position)
    SELECT sqlc.arg(id)::text, t.id, sqlc.arg(title)::text, sqlc.narg(description)::text,
           sqlc.narg(due_date)::date, sqlc.narg(estimated_minutes)::integer,
           sqlc.arg(priority)::text, false, p.position
    FROM owned_task AS t CROSS JOIN next_position AS p
    RETURNING id, task_id, title, description, due_date, estimated_minutes, priority,
              completed, position, (deleted_at IS NOT NULL) AS deleted, created_at, updated_at
)
SELECT * FROM inserted;

-- name: UpdateActionItemByTaskAndUserID :one
UPDATE action_items AS ai
SET title = sqlc.arg(title),
    description = sqlc.narg(description),
    due_date = sqlc.narg(due_date),
    estimated_minutes = sqlc.narg(estimated_minutes),
    priority = sqlc.arg(priority),
    position = sqlc.arg(position),
    updated_at = now()
FROM tasks AS t
WHERE ai.id = sqlc.arg(id)::text
  AND ai.task_id = sqlc.arg(task_id)::text
  AND t.id = ai.task_id
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'update')
  AND ai.deleted_at IS NULL
RETURNING ai.id, ai.task_id, ai.title, ai.description, ai.due_date, ai.estimated_minutes,
          ai.priority, ai.completed, ai.position, (ai.deleted_at IS NOT NULL) AS deleted,
          ai.created_at, ai.updated_at;

-- name: SetActionItemCompletedByTaskAndUserID :one
UPDATE action_items AS ai
SET completed = sqlc.arg(completed), updated_at = now()
FROM tasks AS t
WHERE ai.id = sqlc.arg(id)::text
  AND ai.task_id = sqlc.arg(task_id)::text
  AND t.id = ai.task_id
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'update')
  AND ai.deleted_at IS NULL
RETURNING ai.id;

-- name: UpdateActionItemPositionByTaskAndUserID :one
UPDATE action_items AS ai
SET position = sqlc.arg(position), updated_at = now()
FROM tasks AS t
WHERE ai.id = sqlc.arg(id)::text
  AND ai.task_id = sqlc.arg(task_id)::text
  AND t.id = ai.task_id
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'update')
  AND ai.deleted_at IS NULL
RETURNING ai.id;

-- name: ReorderActionItemsByTaskAndUserID :one
WITH target AS (
    SELECT ai.id, ai.task_id, ai.position AS old_position,
           sqlc.arg(position)::integer AS requested_position,
           (SELECT count(*) FROM action_items all_items
            WHERE all_items.task_id = ai.task_id AND all_items.deleted_at IS NULL) AS group_size,
           (SELECT count(*) FROM action_items before_items
            WHERE before_items.task_id = ai.task_id AND before_items.deleted_at IS NULL
              AND (before_items.position, before_items.id) < (ai.position, ai.id)) AS old_rank
    FROM action_items ai JOIN tasks t ON t.id = ai.task_id
    WHERE ai.id = sqlc.arg(id)::text AND ai.task_id = sqlc.arg(task_id)::text
      AND t.deleted_at IS NULL
      AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
      AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'update')
      AND ai.deleted_at IS NULL
    FOR UPDATE OF ai
), locked_group AS (
    SELECT ai.id, ai.task_id, ai.position, target.id AS target_id,
           target.requested_position, target.old_rank AS target_rank
    FROM action_items ai CROSS JOIN target
    WHERE ai.task_id = target.task_id AND ai.deleted_at IS NULL
    FOR UPDATE OF ai
), ranked_group AS (
    SELECT locked_group.*,
           row_number() OVER (ORDER BY locked_group.position, locked_group.id)::integer - 1 AS old_rank
    FROM locked_group
), reordered AS (
    SELECT ranked_group.id,
           CASE WHEN ranked_group.id = ranked_group.target_id THEN ranked_group.requested_position
                ELSE CASE
                    WHEN ranked_group.old_rank - CASE WHEN ranked_group.old_rank > ranked_group.target_rank THEN 1 ELSE 0 END >= target.requested_position
                    THEN ranked_group.old_rank - CASE WHEN ranked_group.old_rank > ranked_group.target_rank THEN 1 ELSE 0 END + 1
                    ELSE ranked_group.old_rank - CASE WHEN ranked_group.old_rank > ranked_group.target_rank THEN 1 ELSE 0 END
                END
           END AS new_position
    FROM ranked_group JOIN target ON true
    WHERE target.requested_position >= 0 AND target.requested_position < target.group_size
), updated AS (
    UPDATE action_items ai SET position = reordered.new_position, updated_at = now()
    FROM reordered WHERE ai.id = reordered.id
    RETURNING ai.id, ai.task_id, ai.title, ai.description, ai.due_date, ai.estimated_minutes,
              ai.priority, ai.completed, ai.position, (ai.deleted_at IS NOT NULL) AS deleted,
              ai.created_at, ai.updated_at
)
SELECT updated.* FROM updated JOIN target ON target.id = updated.id;

-- name: DeleteActionItemByTaskAndUserID :one
UPDATE action_items AS ai
SET deleted_at = COALESCE(ai.deleted_at, now()), updated_at = now()
FROM tasks AS t
WHERE ai.id = sqlc.arg(id)::text
  AND ai.task_id = sqlc.arg(task_id)::text
  AND t.id = ai.task_id
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'delete')
  AND ai.deleted_at IS NULL
RETURNING ai.id;
