-- name: CreateTaskTag :one
INSERT INTO task_tags (id, user_id, name)
VALUES (sqlc.arg(id)::text, sqlc.arg(user_id)::text, sqlc.arg(name)::citext)
RETURNING id, user_id, name, created_at, updated_at;

-- name: RenameTaskTagByUserID :one
UPDATE task_tags
SET name = sqlc.arg(name)::citext,
    updated_at = now()
WHERE id = sqlc.arg(id)::text
  AND user_id = sqlc.arg(user_id)::text
RETURNING id, user_id, name, created_at, updated_at;

-- name: DeleteTaskTagByUserID :execrows
DELETE FROM task_tags
WHERE id = sqlc.arg(id)::text
  AND user_id = sqlc.arg(user_id)::text;

-- name: AddTaskTagToTaskByUserID :one
WITH owned_pair AS (
    SELECT t.id AS task_id, tt.id AS tag_id
    FROM tasks AS t
    JOIN task_tags AS tt ON tt.user_id = t.user_id
    WHERE t.id = sqlc.arg(task_id)::text
      AND tt.id = sqlc.arg(tag_id)::text
      AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'task', 'update')
), inserted AS (
    INSERT INTO task_tag_assignments (task_id, tag_id)
    SELECT task_id, tag_id FROM owned_pair
    ON CONFLICT (task_id, tag_id) DO NOTHING
    RETURNING 1
)
SELECT EXISTS (SELECT 1 FROM owned_pair) AS owned,
       EXISTS (SELECT 1 FROM inserted) AS added;

-- name: RemoveTaskTagFromTaskByUserID :one
WITH owned_pair AS (
    SELECT t.id AS task_id, tt.id AS tag_id
    FROM tasks AS t
    JOIN task_tags AS tt ON tt.user_id = t.user_id
    WHERE t.id = sqlc.arg(task_id)::text
      AND tt.id = sqlc.arg(tag_id)::text
      AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'task', 'update')
), deleted AS (
    DELETE FROM task_tag_assignments AS assignment
    USING owned_pair
    WHERE assignment.task_id = owned_pair.task_id
      AND assignment.tag_id = owned_pair.tag_id
    RETURNING 1
)
SELECT EXISTS (SELECT 1 FROM owned_pair) AS owned,
       EXISTS (SELECT 1 FROM deleted) AS removed;
