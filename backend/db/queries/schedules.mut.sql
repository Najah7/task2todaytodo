-- name: CreateScheduleByUserID :one
WITH project_scope AS (
    SELECT p.id, p.user_id
    FROM projects p
    WHERE p.id = sqlc.narg(project_id)::text
      AND project_has_permission(p.id, sqlc.arg(actor_id)::text, 'schedule', 'create')
    UNION ALL
    SELECT NULL::text, sqlc.arg(actor_id)::text
    WHERE sqlc.narg(project_id)::text IS NULL
), inserted AS (
    INSERT INTO schedules (
        id, user_id, project_id, assignee_id, title, description, location,
        series_id, occurrence_date, timezone, is_exception, start_at, end_at,
        repeat_state, frequency_anchor_date, interval_weeks, changed_by
    )
    SELECT sqlc.arg(id)::text, scope.user_id, scope.id, scope.user_id,
           sqlc.arg(title)::text, sqlc.narg(description)::text, sqlc.narg(location)::text,
           sqlc.arg(series_id)::text, sqlc.arg(occurrence_date)::date, sqlc.arg(timezone)::text,
           sqlc.arg(is_exception)::boolean, sqlc.arg(start_at)::timestamptz, sqlc.arg(end_at)::timestamptz,
           CASE WHEN sqlc.arg(interval_weeks)::integer > 0 THEN 'active' ELSE 'one_off' END,
           CASE WHEN sqlc.arg(interval_weeks)::integer > 0 THEN sqlc.arg(occurrence_date)::date ELSE NULL END,
           sqlc.arg(interval_weeks)::integer, sqlc.arg(actor_id)::text
    FROM project_scope scope
    RETURNING *
), inserted_frequencies AS (
    INSERT INTO schedule_frequencies(schedule_id, frequency)
    SELECT i.id, unnest(sqlc.arg(frequencies)::text[])
    FROM inserted i
    WHERE i.id = i.series_id AND i.interval_weeks > 0
    ON CONFLICT DO NOTHING
    RETURNING schedule_id, frequency
), initial_history AS (
    INSERT INTO schedule_revisions (
        id, revision, user_id, project_id, assignee_id, title, description, location,
        start_at, end_at, series_id, occurrence_date, timezone, is_exception,
        repeat_state, frequency_anchor_date, interval_weeks, completed, deleted_at,
        skipped_at, frequencies, created_at, updated_at, changed_by, changed_at
    )
    SELECT i.id, i.revision, i.user_id, i.project_id, i.assignee_id, i.title, i.description, i.location,
           i.start_at, i.end_at, i.series_id, i.occurrence_date, i.timezone, i.is_exception,
           i.repeat_state, i.frequency_anchor_date, i.interval_weeks, i.completed, i.deleted_at,
           i.skipped_at,
           COALESCE(ARRAY(
               SELECT f.frequency FROM inserted_frequencies f
               WHERE f.schedule_id = i.id ORDER BY f.frequency
           ), '{}'::text[]),
           i.created_at, i.updated_at, i.changed_by, i.updated_at
    FROM inserted i
    WHERE i.id = i.series_id AND i.repeat_state = 'active'
    RETURNING id
)
SELECT i.id, i.user_id, i.project_id, i.assignee_id, i.title, i.description, i.location,
       i.interval_weeks, i.repeat_state, i.frequency_anchor_date,
       i.series_id, i.occurrence_date, i.timezone, i.is_exception, i.completed,
       false AS deleted,
       ARRAY(SELECT f.frequency FROM schedule_frequencies f WHERE f.schedule_id = i.series_id ORDER BY f.frequency)::text[] AS frequencies,
       i.start_at, i.end_at, i.created_at, i.updated_at, i.revision, i.changed_by
FROM inserted i;

-- name: UpdateScheduleByUserID :one
WITH updated AS (
    UPDATE schedules s
    SET title = sqlc.arg(title)::text,
        description = sqlc.narg(description)::text,
        location = sqlc.narg(location)::text,
        start_at = sqlc.arg(start_at)::timestamptz,
        end_at = sqlc.arg(end_at)::timestamptz,
        is_exception = CASE WHEN s.id = s.series_id THEN s.is_exception ELSE true END,
        changed_by = sqlc.arg(actor_id)::text,
        updated_at = now()
    WHERE s.id = sqlc.arg(id)::text
      AND s.deleted_at IS NULL
      AND schedule_has_permission(s.id, sqlc.arg(actor_id)::text, 'schedule', 'update')
    RETURNING s.*
)
SELECT u.id, u.user_id, u.project_id, u.assignee_id, u.title, u.description, u.location,
       r.interval_weeks, r.repeat_state, r.frequency_anchor_date,
       u.series_id, u.occurrence_date, u.timezone, u.is_exception, u.completed,
       false AS deleted,
       ARRAY(SELECT f.frequency FROM schedule_frequencies f WHERE f.schedule_id = u.series_id ORDER BY f.frequency)::text[] AS frequencies,
       u.start_at, u.end_at, u.created_at, u.updated_at, u.revision, u.changed_by
FROM updated u JOIN schedules r ON r.id = u.series_id AND r.user_id = u.user_id;

-- name: DeleteScheduleByUserID :one
UPDATE schedules s
SET deleted_at = COALESCE(s.deleted_at, now()), changed_by = sqlc.arg(actor_id)::text, updated_at = now()
WHERE s.id = sqlc.arg(id)::text
  AND s.deleted_at IS NULL
  AND schedule_has_permission(s.id, sqlc.arg(actor_id)::text, 'schedule', 'delete')
RETURNING s.id;

-- name: DeleteProjectSchedulesByActor :execrows
UPDATE schedules AS s
SET deleted_at = now(),
    changed_by = sqlc.arg(actor_id)::text,
    updated_at = now()
WHERE s.project_id = sqlc.arg(project_id)::text
  AND s.deleted_at IS NULL;

-- name: ReassignProjectMemberSchedulesByActor :execrows
UPDATE schedules AS s
SET assignee_id = p.user_id,
    changed_by = sqlc.arg(actor_id)::text,
    updated_at = now()
FROM projects AS p
WHERE s.project_id = p.id
  AND p.id = sqlc.arg(project_id)::text
  AND s.assignee_id = sqlc.arg(member_id)::text;

-- name: SetScheduleCompletedByUserID :execrows
UPDATE schedules s
SET completed = sqlc.arg(completed)::boolean,
    is_exception = CASE WHEN s.id = s.series_id THEN s.is_exception ELSE true END,
    changed_by = sqlc.arg(actor_id)::text,
    updated_at = now()
WHERE s.id = sqlc.arg(id)::text
  AND s.deleted_at IS NULL
  AND schedule_has_permission(s.id, sqlc.arg(actor_id)::text, 'schedule', 'update');

-- name: UpdateScheduleProjectByUserID :execrows
WITH locked AS (
    SELECT root.id, root.user_id, root.project_id
    FROM schedules root
    WHERE root.id = sqlc.arg(series_id)::text
      AND root.id = root.series_id
      AND root.deleted_at IS NULL
      AND schedule_has_permission(root.id, sqlc.arg(actor_id)::text, 'schedule', 'update')
    FOR UPDATE
), target AS (
    SELECT p.id
    FROM projects p, locked l
    WHERE p.id = sqlc.narg(project_id)::text
      AND p.user_id = l.user_id
      AND p.deleted_at IS NULL
      AND project_has_permission(p.id, sqlc.arg(actor_id)::text, 'schedule', 'create')
    UNION ALL
    SELECT NULL::text
    FROM locked l
    WHERE sqlc.narg(project_id)::text IS NULL
      AND (l.project_id IS NULL OR project_has_permission(l.project_id, sqlc.arg(actor_id)::text, 'schedule', 'update'))
)
UPDATE schedules s
SET project_id = target.id,
    assignee_id = CASE WHEN s.assignee_id = s.user_id OR EXISTS (
        SELECT 1 FROM project_members pm WHERE pm.project_id = target.id AND pm.user_id = s.assignee_id
    ) THEN s.assignee_id ELSE s.user_id END,
    changed_by = sqlc.arg(actor_id)::text,
    updated_at = now()
FROM locked, target
WHERE s.series_id = locked.id
  AND s.user_id = locked.user_id;

-- name: UpdateScheduleAssigneeByUserID :execrows
WITH eligible_root AS (
    SELECT root.id, root.project_id, root.user_id
    FROM schedules root
    WHERE root.id = sqlc.arg(series_id)::text
      AND root.id = root.series_id
      AND root.deleted_at IS NULL
      AND schedule_has_permission(root.id, sqlc.arg(actor_id)::text, 'schedule_assignment', 'update')
      AND (
          sqlc.arg(assignee_id)::text = root.user_id
          OR EXISTS (SELECT 1 FROM project_members pm WHERE pm.project_id = root.project_id AND pm.user_id = sqlc.arg(assignee_id)::text)
      )
)
UPDATE schedules s
SET assignee_id = sqlc.arg(assignee_id)::text,
    changed_by = sqlc.arg(actor_id)::text,
    updated_at = now()
FROM eligible_root root
WHERE s.series_id = root.id
  AND s.user_id = root.user_id;

-- name: CreateScheduleOccurrenceByUserID :one
INSERT INTO schedules (
    id, user_id, project_id, assignee_id, title, description, location,
    series_id, occurrence_date, timezone, is_exception, start_at, end_at,
    repeat_state, frequency_anchor_date, interval_weeks, changed_by
)
SELECT sqlc.arg(id)::text, root.user_id, root.project_id, root.assignee_id,
       root.title, root.description, root.location,
       root.id, sqlc.arg(occurrence_date)::date, sqlc.arg(timezone)::text,
       sqlc.arg(is_exception)::boolean, sqlc.arg(start_at)::timestamptz, sqlc.arg(end_at)::timestamptz,
       NULL, NULL, 0, sqlc.arg(user_id)::text
FROM schedules root
WHERE root.id = sqlc.arg(series_id)::text
  AND root.id = root.series_id
  AND root.user_id = sqlc.arg(user_id)::text
  AND root.repeat_state = 'active'
  AND root.frequency_anchor_date <= sqlc.arg(occurrence_date)::date
  AND root.deleted_at IS NULL
  AND (root.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = root.project_id AND p.deleted_at IS NULL))
ON CONFLICT (series_id, occurrence_date) WHERE id <> series_id DO UPDATE
SET id = schedules.id
WHERE schedules.deleted_at IS NULL
RETURNING id;

-- name: TombstoneScheduleByUserID :one
UPDATE schedules s
SET deleted_at = COALESCE(s.deleted_at, now()), changed_by = sqlc.arg(actor_id)::text, updated_at = now()
WHERE s.id = sqlc.arg(id)::text
  AND s.deleted_at IS NULL
  AND schedule_has_permission(s.id, sqlc.arg(actor_id)::text, 'schedule', 'delete')
RETURNING s.id;

-- name: DeleteUneditedFutureSchedulesBySeries :execrows
UPDATE schedules s
SET deleted_at = COALESCE(s.deleted_at, now()), changed_by = sqlc.arg(user_id)::text, updated_at = now()
WHERE s.series_id = sqlc.arg(series_id)::text
  AND s.id <> s.series_id
  AND s.start_at > sqlc.arg(from_at)::timestamptz
  AND s.is_exception = false
  AND s.completed = false
  AND s.deleted_at IS NULL
  AND schedule_has_permission(s.id, sqlc.arg(user_id)::text, 'schedule', 'update');

-- name: DeleteUneditedFutureSchedulesByUserID :execrows
UPDATE schedules s
SET deleted_at = COALESCE(s.deleted_at, now()), changed_by = sqlc.arg(user_id)::text, updated_at = now()
WHERE s.user_id = sqlc.arg(user_id)::text
  AND s.id <> s.series_id
  AND s.start_at > sqlc.arg(from_at)::timestamptz
  AND s.is_exception = false
  AND s.completed = false
  AND s.deleted_at IS NULL
  AND schedule_has_permission(s.id, sqlc.arg(user_id)::text, 'schedule', 'update');
