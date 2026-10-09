-- name: CreateActionItem :one
WITH inserted AS (
    INSERT INTO action_items (
        id, task_id, title, description, due_date, completed, position,
        series_id, occurrence_date, timezone, is_exception, repeat_state,
        frequency_anchor_date, interval_weeks
    )
    VALUES (sqlc.arg(id), sqlc.arg(task_id), sqlc.arg(title), sqlc.narg(description),
        sqlc.narg(due_date), sqlc.arg(completed), sqlc.arg(position),
        sqlc.arg(series_id), sqlc.arg(occurrence_date), sqlc.arg(timezone), sqlc.arg(is_exception),
        CASE WHEN sqlc.arg(interval_weeks)::integer > 0 THEN 'active' ELSE 'one_off' END,
        CASE WHEN sqlc.arg(interval_weeks)::integer > 0 THEN sqlc.arg(occurrence_date)::date ELSE NULL END,
        sqlc.arg(interval_weeks)::integer)
    RETURNING id, task_id, title, description, due_date, completed, position,
        series_id, occurrence_date, timezone, is_exception, (deleted_at IS NOT NULL) AS deleted,
        created_at, updated_at
)
SELECT inserted.*, sqlc.arg(interval_weeks)::integer AS interval_weeks FROM inserted;

-- name: CreateActionItemByTaskAndUserID :one
WITH owned_task AS (
    SELECT tasks.id
    FROM tasks
    WHERE tasks.id = sqlc.arg(task_id)
      AND tasks.deleted_at IS NULL
      AND (tasks.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = tasks.project_id AND p.deleted_at IS NULL))
      AND task_has_permission(tasks.id, sqlc.arg(user_id)::text, 'action_item', 'create')
      AND tasks.status <> 'done'
),
next_position AS (
    SELECT COALESCE(sqlc.narg(position)::integer, COALESCE(MAX(ti.position), -1)::integer + 1) AS position
    FROM owned_task AS ot
    LEFT JOIN action_items AS ti ON ti.task_id = ot.id
      AND ti.occurrence_date = sqlc.arg(occurrence_date)::date
      AND ti.deleted_at IS NULL
),
inserted AS (
    INSERT INTO action_items (
        id, task_id, title, description, due_date, completed, position,
        series_id, occurrence_date, timezone, is_exception, repeat_state,
        frequency_anchor_date, interval_weeks
    )
    SELECT
        sqlc.arg(id),
        owned_task.id,
        sqlc.arg(title),
        sqlc.arg(description),
        sqlc.arg(due_date),
        false,
        next_position.position,
        sqlc.arg(series_id),
        sqlc.arg(occurrence_date),
        sqlc.arg(timezone),
        sqlc.arg(is_exception),
        CASE WHEN sqlc.arg(interval_weeks)::integer > 0 THEN 'active' ELSE 'one_off' END,
        CASE WHEN sqlc.arg(interval_weeks)::integer > 0 THEN sqlc.arg(occurrence_date)::date ELSE NULL END,
        sqlc.arg(interval_weeks)::integer
    FROM owned_task
    CROSS JOIN next_position
    RETURNING id, task_id, title, description, due_date, completed, position,
        series_id, occurrence_date, timezone, is_exception, interval_weeks, (deleted_at IS NOT NULL) AS deleted,
        created_at, updated_at
),
inserted_frequencies AS (
    INSERT INTO action_item_frequencies (action_item_id, frequency)
    SELECT inserted.id, unnest(sqlc.arg(frequencies)::text[])
    FROM inserted
    WHERE inserted.id = inserted.series_id AND inserted.interval_weeks > 0
    ON CONFLICT DO NOTHING
)
SELECT
    inserted.id,
    inserted.task_id,
    inserted.title,
    inserted.description,
    inserted.due_date,
    inserted.completed,
    inserted.position,
    sqlc.arg(interval_weeks)::integer AS interval_weeks,
    inserted.series_id,
    inserted.occurrence_date,
    inserted.timezone,
    inserted.is_exception,
    inserted.deleted,
    ARRAY(
        SELECT tif.frequency
        FROM action_item_frequencies AS tif
        WHERE tif.action_item_id = inserted.id
        ORDER BY tif.frequency
    )::text[] AS frequencies,
    inserted.created_at,
    inserted.updated_at
FROM inserted;

-- name: CreateActionItemOccurrenceByTaskAndUserID :one
WITH inserted AS (
    INSERT INTO action_items (
        id, task_id, title, description, due_date, completed, position,
        series_id, occurrence_date, timezone, is_exception, repeat_state,
        frequency_anchor_date, interval_weeks
    )
    SELECT
        sqlc.arg(id), source.task_id,
        source.title,
        source.description,
        sqlc.arg(due_date), false,
        source.position,
        source.series_id, sqlc.arg(occurrence_date), source.timezone, false, NULL, NULL, 0
    FROM action_items AS source
    JOIN tasks AS t ON t.id = source.task_id
    WHERE source.id = sqlc.arg(series_id)
      AND source.series_id = source.id
      AND source.repeat_state = 'active'
      AND source.frequency_anchor_date <= sqlc.arg(occurrence_date)::date
      AND source.deleted_at IS NULL
      AND t.id = source.task_id
      AND t.user_id = sqlc.arg(user_id)
      AND t.deleted_at IS NULL
      AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
      AND t.status <> 'done'
    ON CONFLICT (series_id, occurrence_date) WHERE id <> series_id DO NOTHING
    RETURNING id, task_id, title, description, due_date, completed, position,
        series_id, occurrence_date, timezone, is_exception, (deleted_at IS NOT NULL) AS deleted,
        created_at, updated_at
),
candidate AS (
    SELECT * FROM inserted
    UNION ALL
    SELECT existing.id, existing.task_id, existing.title, existing.description, existing.due_date,
        existing.completed, existing.position, existing.series_id,
        existing.occurrence_date, existing.timezone, existing.is_exception,
        (existing.deleted_at IS NOT NULL) AS deleted, existing.created_at, existing.updated_at
    FROM action_items AS existing
    JOIN tasks AS t ON t.id = existing.task_id
    WHERE existing.series_id = sqlc.arg(series_id)
      AND existing.occurrence_date = sqlc.arg(occurrence_date)
      AND existing.id <> existing.series_id
      AND existing.deleted_at IS NULL
      AND t.user_id = sqlc.arg(user_id)
      AND t.deleted_at IS NULL
      AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
      AND t.status <> 'done'
      AND NOT EXISTS (SELECT 1 FROM inserted)
    ORDER BY existing.id
    LIMIT 1
)
SELECT
    candidate.id,
    candidate.task_id,
    candidate.title,
    candidate.description,
    candidate.due_date,
    candidate.completed,
    candidate.position,
    COALESCE((SELECT r.interval_weeks FROM action_items r WHERE r.id = candidate.series_id ), 0)::integer AS interval_weeks,
    candidate.series_id,
    candidate.occurrence_date,
    candidate.timezone,
    candidate.is_exception,
    candidate.deleted,
    ARRAY(
        SELECT tif.frequency
        FROM action_item_frequencies AS tif
        WHERE tif.action_item_id = candidate.series_id
        ORDER BY tif.frequency
    )::text[] AS frequencies,
    candidate.created_at,
    candidate.updated_at
FROM candidate;

-- name: UpdateActionItem :one
UPDATE action_items
SET task_id = $2,
    title = $3,
    description = $4,
    due_date = $5,
    completed = $6,
    position = $7,
    series_id = $8,
    occurrence_date = $9,
    timezone = $10,
    is_exception = $11,
    updated_at = now()
WHERE action_items.id = $1
  AND action_items.deleted_at IS NULL
RETURNING id, task_id, title, description, due_date, completed, position, COALESCE((SELECT r.interval_weeks FROM action_items r WHERE r.id = action_items.series_id ), 0)::integer AS interval_weeks,
    series_id, occurrence_date, timezone, is_exception, (deleted_at IS NOT NULL) AS deleted,
    created_at, updated_at;

-- name: UpdateActionItemByTaskAndUserID :one
WITH updated AS (
    UPDATE action_items AS ti
    SET title = sqlc.arg(title),
        description = sqlc.arg(description),
        due_date = sqlc.arg(due_date),
        position = sqlc.arg(position),
        is_exception = true,
        updated_at = now()
    FROM tasks AS t
    WHERE ti.id = sqlc.arg(id)
      AND ti.task_id = sqlc.arg(task_id)
      AND t.id = ti.task_id
      AND t.deleted_at IS NULL
      AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
      AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'update')
      AND ti.deleted_at IS NULL
    RETURNING ti.id, ti.task_id, ti.title, ti.description, ti.due_date, ti.completed, ti.position,
        COALESCE((SELECT r.interval_weeks FROM action_items r WHERE r.id = ti.series_id ), 0)::integer AS interval_weeks, ti.series_id, ti.occurrence_date, ti.timezone, ti.is_exception,
        (ti.deleted_at IS NOT NULL) AS deleted, ti.created_at, ti.updated_at
)
SELECT
    updated.id,
    updated.task_id,
    updated.title,
    updated.description,
    updated.due_date,
    updated.completed,
    updated.position,
    COALESCE((SELECT r.interval_weeks FROM action_items r WHERE r.id = updated.series_id ), 0)::integer AS interval_weeks,
    updated.series_id,
    updated.occurrence_date,
    updated.timezone,
    updated.is_exception,
    updated.deleted,
    ARRAY(
        SELECT tif.frequency
        FROM action_item_frequencies AS tif
        WHERE tif.action_item_id = updated.series_id
        ORDER BY tif.frequency
    )::text[] AS frequencies,
    updated.created_at,
    updated.updated_at
FROM updated;

-- name: SetActionItemCompletedByTaskAndUserID :one
UPDATE action_items AS ti
SET completed = $4,
    is_exception = CASE WHEN ti.id = ti.series_id THEN ti.is_exception ELSE true END,
    updated_at = now()
FROM tasks AS t
WHERE ti.id = $1
  AND ti.task_id = $2
  AND t.id = ti.task_id
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, $3, 'action_item', 'update')
  AND ti.deleted_at IS NULL
RETURNING ti.id, ti.task_id, ti.title, ti.description, ti.due_date, ti.completed, ti.position,
    COALESCE((SELECT r.interval_weeks FROM action_items r WHERE r.id = ti.series_id ), 0)::integer AS interval_weeks, ti.series_id, ti.occurrence_date, ti.timezone, ti.is_exception,
    (ti.deleted_at IS NOT NULL) AS deleted, ti.created_at, ti.updated_at;

-- name: UpdateActionItemPositionByTaskAndUserID :one
UPDATE action_items AS ti
SET position = sqlc.arg(position),
    is_exception = CASE WHEN ti.id = ti.series_id THEN ti.is_exception ELSE true END,
    updated_at = now()
FROM tasks AS t
WHERE ti.id = sqlc.arg(id)
  AND ti.task_id = sqlc.arg(task_id)
  AND t.id = ti.task_id
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'update')
  AND ti.deleted_at IS NULL
RETURNING ti.id;

-- name: ReorderActionItemsByTaskAndUserID :one
WITH target AS (
    SELECT ti.id, ti.task_id, ti.occurrence_date, ti.position AS old_position,
        sqlc.arg(position)::integer AS requested_position,
        (SELECT count(*) FROM action_items AS all_items
         WHERE all_items.task_id = ti.task_id
           AND all_items.occurrence_date = ti.occurrence_date
           AND all_items.deleted_at IS NULL) AS group_size,
        (SELECT count(*) FROM action_items AS before_items
         WHERE before_items.task_id = ti.task_id
           AND before_items.occurrence_date = ti.occurrence_date
           AND before_items.deleted_at IS NULL
           AND before_items.position < ti.position) AS old_rank
    FROM action_items AS ti
    JOIN tasks AS t ON t.id = ti.task_id
    WHERE ti.id = sqlc.arg(id)
      AND ti.task_id = sqlc.arg(task_id)
      AND t.deleted_at IS NULL
      AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
      AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'update')
      AND ti.deleted_at IS NULL
    FOR UPDATE OF ti
), locked_group AS (
    SELECT ti.id, ti.task_id, ti.occurrence_date, ti.position,
        target.id AS target_id,
        target.requested_position,
        target.old_rank AS target_rank
    FROM action_items AS ti
    CROSS JOIN target
    WHERE ti.task_id = target.task_id
      AND ti.occurrence_date = target.occurrence_date
      AND ti.deleted_at IS NULL
    FOR UPDATE OF ti
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
    FROM ranked_group
    JOIN target ON true
    WHERE target.requested_position >= 0 AND target.requested_position < target.group_size
), updated AS (
    UPDATE action_items AS ti
    SET position = reordered.new_position,
        is_exception = CASE WHEN ti.position IS DISTINCT FROM reordered.new_position THEN true ELSE ti.is_exception END,
        updated_at = now()
    FROM reordered
    WHERE ti.id = reordered.id
    RETURNING ti.id, ti.task_id, ti.title, ti.description, ti.due_date, ti.completed, ti.position,
        COALESCE((SELECT r.interval_weeks FROM action_items r WHERE r.id = ti.series_id ), 0)::integer AS interval_weeks, ti.series_id, ti.occurrence_date, ti.timezone, ti.is_exception,
        (ti.deleted_at IS NOT NULL) AS deleted, ti.created_at, ti.updated_at
)
SELECT
    updated.id,
    updated.task_id,
    updated.title,
    updated.description,
    updated.due_date,
    updated.completed,
    updated.position,
    COALESCE((SELECT r.interval_weeks FROM action_items r WHERE r.id = updated.series_id ), 0)::integer AS interval_weeks,
    updated.series_id,
    updated.occurrence_date,
    updated.timezone,
    updated.is_exception,
    updated.deleted,
    ARRAY(
        SELECT tif.frequency
        FROM action_item_frequencies AS tif
        WHERE tif.action_item_id = updated.series_id
        ORDER BY tif.frequency
    )::text[] AS frequencies,
    updated.created_at,
    updated.updated_at
FROM updated
JOIN target ON target.id = updated.id;

-- name: DeleteActionItem :exec
UPDATE action_items
SET deleted_at = COALESCE(deleted_at, now()), updated_at = now()
WHERE id = $1
  AND deleted_at IS NULL;

-- name: DeleteActionItemByTaskAndUserID :one
UPDATE action_items AS ti
SET deleted_at = COALESCE(ti.deleted_at, now()), updated_at = now()
FROM tasks AS t
WHERE ti.id = $1
  AND ti.task_id = $2
  AND t.id = ti.task_id
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, $3, 'action_item', 'delete')
  AND ti.deleted_at IS NULL
RETURNING ti.id;

-- name: TombstoneActionItemByTaskAndUserID :one
UPDATE action_items AS ti
SET deleted_at = COALESCE(ti.deleted_at, now()), updated_at = now()
FROM tasks AS t
WHERE ti.id = sqlc.arg(id)
  AND ti.task_id = sqlc.arg(task_id)
  AND t.id = ti.task_id
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'delete')
  AND ti.deleted_at IS NULL
RETURNING ti.id;

-- name: DeleteUneditedFutureActionItemsBySeries :execrows
UPDATE action_items AS ti
SET deleted_at = COALESCE(ti.deleted_at, now()), updated_at = now()
FROM tasks AS t
WHERE ti.task_id = sqlc.arg(task_id)
  AND t.id = ti.task_id
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'update')
  AND ti.series_id = sqlc.arg(series_id)
  AND ti.id <> ti.series_id
  AND ti.occurrence_date >= sqlc.arg(from_date)
  AND ti.completed = false
  AND ti.is_exception = false
  AND ti.deleted_at IS NULL;

-- name: DeleteUneditedFutureActionItemsByTask :execrows
UPDATE action_items AS ti
SET deleted_at = COALESCE(ti.deleted_at, now()), updated_at = now()
FROM tasks AS t
WHERE ti.task_id = sqlc.arg(task_id)
  AND t.id = ti.task_id
  AND t.deleted_at IS NULL
  AND (t.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id AND p.deleted_at IS NULL))
  AND task_has_permission(t.id, sqlc.arg(user_id)::text, 'action_item', 'update')
  AND ti.id <> ti.series_id
  AND ti.occurrence_date >= (sqlc.arg(from_at)::timestamptz AT TIME ZONE ti.timezone)::date
  AND ti.completed = false
  AND ti.is_exception = false
  AND ti.deleted_at IS NULL;
