-- name: GetScheduleByUserIDForPermission :one
SELECT s.id, s.user_id, s.project_id, s.assignee_id, s.title, s.description, s.location,
       r.interval_weeks, r.repeat_state, r.frequency_anchor_date,
       s.series_id, s.occurrence_date, s.timezone, s.is_exception, s.completed,
       COALESCE((SELECT p.status = 'done' FROM projects p WHERE p.id = s.project_id AND p.deleted_at IS NULL), false) AS project_done,
       (s.deleted_at IS NOT NULL) AS deleted,
       ARRAY(SELECT f.frequency FROM schedule_frequencies f WHERE f.schedule_id = s.series_id ORDER BY f.frequency)::text[] AS frequencies,
       s.start_at, s.end_at, s.created_at, s.updated_at, s.revision, s.changed_by
FROM schedules s
JOIN schedules r ON r.id = s.series_id AND r.user_id = s.user_id
WHERE s.id = sqlc.arg(id)::text
  AND schedule_has_permission(s.id, sqlc.arg(actor_id)::text, sqlc.arg(resource_id)::text, sqlc.arg(action)::action)
  AND s.deleted_at IS NULL;

-- name: GetScheduleProjectByUserIDForPermission :one
SELECT s.project_id
FROM schedules AS s
WHERE s.id = sqlc.arg(series_id)::text
  AND s.id = s.series_id
  AND s.deleted_at IS NULL
  AND schedule_has_permission(s.id, sqlc.arg(actor_id)::text, sqlc.arg(resource_id)::text, sqlc.arg(action)::action);

-- name: LockScheduleByUserIDForPermission :one
SELECT s.project_id
FROM schedules AS s
WHERE s.id = sqlc.arg(series_id)::text
  AND s.id = s.series_id
  AND s.deleted_at IS NULL
  AND schedule_has_permission(s.id, sqlc.arg(actor_id)::text, sqlc.arg(resource_id)::text, sqlc.arg(action)::action)
FOR UPDATE;

-- name: LockActiveProjectForScheduleMutation :one
SELECT p.id
FROM projects AS p
WHERE p.id = sqlc.arg(project_id)::text
  AND p.deleted_at IS NULL
FOR UPDATE;

-- name: ListSchedulesForOccurrenceProjectionByAssigneeUserID :many
SELECT s.id, s.user_id, s.project_id, s.assignee_id, s.title, s.description, s.location,
       r.interval_weeks, r.repeat_state, r.frequency_anchor_date,
       s.series_id, s.occurrence_date, s.timezone, s.is_exception, s.completed,
       COALESCE((SELECT p.status = 'done' FROM projects p WHERE p.id = s.project_id AND p.deleted_at IS NULL), false) AS project_done,
       (s.deleted_at IS NOT NULL) AS deleted,
       ARRAY(SELECT f.frequency FROM schedule_frequencies f WHERE f.schedule_id = s.series_id ORDER BY f.frequency)::text[] AS frequencies,
       s.start_at, s.end_at, s.created_at, s.updated_at, s.revision, s.changed_by
FROM schedules s
JOIN schedules r ON r.id = s.series_id AND r.user_id = s.user_id
WHERE s.assignee_id = sqlc.arg(user_id)::text
  AND (s.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = s.project_id AND p.deleted_at IS NULL))
  AND (
      s.user_id = sqlc.arg(user_id)::text
      OR (
          s.project_id IS NOT NULL
          AND EXISTS (SELECT 1 FROM projects p WHERE p.id = s.project_id AND p.deleted_at IS NULL)
          AND project_has_permission(s.project_id, sqlc.arg(user_id)::text, 'schedule', 'read')
      )
  )
ORDER BY s.start_at, s.id;

-- name: ListSchedulesForOccurrenceProjectionByProjectAndUserID :many
SELECT s.id, s.user_id, s.project_id, s.assignee_id, s.title, s.description, s.location,
       r.interval_weeks, r.repeat_state, r.frequency_anchor_date,
       s.series_id, s.occurrence_date, s.timezone, s.is_exception, s.completed,
       COALESCE((SELECT p.status = 'done' FROM projects p WHERE p.id = s.project_id AND p.deleted_at IS NULL), false) AS project_done,
       (s.deleted_at IS NOT NULL) AS deleted,
       ARRAY(SELECT f.frequency FROM schedule_frequencies f WHERE f.schedule_id = s.series_id ORDER BY f.frequency)::text[] AS frequencies,
       s.start_at, s.end_at, s.created_at, s.updated_at, s.revision, s.changed_by
FROM schedules s
JOIN schedules r ON r.id = s.series_id AND r.user_id = s.user_id
WHERE s.project_id = sqlc.arg(project_id)::text
  AND EXISTS (SELECT 1 FROM projects p WHERE p.id = s.project_id AND p.deleted_at IS NULL)
  AND (
      s.user_id = sqlc.arg(user_id)::text
      OR (
          EXISTS (SELECT 1 FROM projects p WHERE p.id = s.project_id AND p.deleted_at IS NULL)
          AND project_has_permission(s.project_id, sqlc.arg(user_id)::text, 'schedule', 'read')
      )
  )
ORDER BY s.start_at, s.id;

-- name: ListSchedulesForOccurrenceProjectionByUserID :many
SELECT s.id, s.user_id, s.project_id, s.assignee_id, s.title, s.description, s.location,
       r.interval_weeks, r.repeat_state, r.frequency_anchor_date,
       s.series_id, s.occurrence_date, s.timezone, s.is_exception, s.completed,
       (s.deleted_at IS NOT NULL) AS deleted,
       ARRAY(SELECT f.frequency FROM schedule_frequencies f WHERE f.schedule_id = s.series_id ORDER BY f.frequency)::text[] AS frequencies,
       s.start_at, s.end_at, s.created_at, s.updated_at, s.revision, s.changed_by
FROM schedules s
JOIN schedules r ON r.id = s.series_id AND r.user_id = s.user_id
WHERE s.assignee_id = sqlc.arg(user_id)::text
  AND (s.user_id = sqlc.arg(user_id)::text OR (
      s.project_id IS NOT NULL
      AND EXISTS (SELECT 1 FROM projects p WHERE p.id = s.project_id AND p.deleted_at IS NULL)
      AND project_has_permission(s.project_id, sqlc.arg(user_id)::text, 'schedule', 'read')
  ))
ORDER BY s.start_at, s.occurrence_date, s.id;

-- name: ListSchedulesForOccurrenceCommandByUserID :many
SELECT s.id, s.user_id, s.project_id, s.assignee_id, s.title, s.description, s.location,
       r.interval_weeks, r.repeat_state, r.frequency_anchor_date,
       s.series_id, s.occurrence_date, s.timezone, s.is_exception, s.completed,
       (s.deleted_at IS NOT NULL) AS deleted,
       ARRAY(SELECT f.frequency FROM schedule_frequencies f WHERE f.schedule_id = s.series_id ORDER BY f.frequency)::text[] AS frequencies,
       s.start_at, s.end_at, s.created_at, s.updated_at, s.revision, s.changed_by
FROM schedules s
JOIN schedules r ON r.id = s.series_id AND r.user_id = s.user_id
WHERE s.series_id = sqlc.arg(series_id)::text
  AND schedule_has_permission(s.id, sqlc.arg(actor_id)::text, sqlc.arg(resource_id)::text, sqlc.arg(action)::action)
ORDER BY s.start_at, s.occurrence_date, s.id;

-- name: ListActiveScheduleSeriesByAssigneeUserID :many
SELECT s.id, s.user_id, s.project_id, s.assignee_id, s.title, s.description, s.location,
       s.interval_weeks, s.repeat_state, s.frequency_anchor_date,
       s.series_id, s.occurrence_date, s.timezone, s.is_exception, s.completed,
       (s.deleted_at IS NOT NULL) AS deleted,
       ARRAY(SELECT f.frequency FROM schedule_frequencies f WHERE f.schedule_id = s.id ORDER BY f.frequency)::text[] AS frequencies,
       s.start_at, s.end_at, s.created_at, s.updated_at, s.revision, s.changed_by
FROM schedules s
WHERE s.id = s.series_id AND s.assignee_id = sqlc.arg(user_id)::text
  AND s.repeat_state = 'active' AND s.deleted_at IS NULL
  AND (s.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = s.project_id AND p.deleted_at IS NULL))
  AND (s.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = s.project_id AND p.deleted_at IS NULL AND p.status <> 'done'))
  AND schedule_has_permission(s.id, sqlc.arg(user_id)::text, 'schedule', 'read')
ORDER BY s.id;

-- name: ListScheduleSkippedOccurrencesByUserID :many
SELECT child.occurrence_date
FROM schedules child
JOIN schedules root ON root.id = child.series_id AND root.user_id = child.user_id AND root.series_id = root.id
WHERE child.series_id = sqlc.arg(series_id)::text
  AND child.id <> child.series_id AND child.deleted_at IS NULL AND child.skipped_at IS NOT NULL
  AND root.deleted_at IS NULL
  AND schedule_has_permission(root.id, sqlc.arg(user_id)::text, 'schedule', 'read')
ORDER BY child.occurrence_date;

-- name: ListScheduleSkippedOccurrencesForPermission :many
SELECT child.occurrence_date
FROM schedules child
JOIN schedules root ON root.id = child.series_id AND root.user_id = child.user_id AND root.series_id = root.id
WHERE child.series_id = sqlc.arg(series_id)::text
  AND child.id <> child.series_id AND child.deleted_at IS NULL AND child.skipped_at IS NOT NULL
  AND root.deleted_at IS NULL
  AND schedule_has_permission(root.id, sqlc.arg(actor_id)::text, sqlc.arg(resource_id)::text, sqlc.arg(action)::action)
ORDER BY child.occurrence_date;

-- name: ListEligibleScheduleAssignees :many
SELECT u.id, u.first_name, u.last_name, u.email, (u.id = s.user_id) AS is_owner
FROM schedules s
JOIN users u ON u.id = s.user_id
WHERE s.id = sqlc.arg(id)::text
  AND schedule_has_permission(s.id, sqlc.arg(actor_id)::text, 'schedule_assignment', 'update')
UNION
SELECT u.id, u.first_name, u.last_name, u.email, false AS is_owner
FROM schedules s
JOIN project_members pm ON pm.project_id = s.project_id
JOIN users u ON u.id = pm.user_id
WHERE s.id = sqlc.arg(id)::text
  AND s.project_id IS NOT NULL
  AND schedule_has_permission(s.id, sqlc.arg(actor_id)::text, 'schedule_assignment', 'update')
ORDER BY is_owner DESC, last_name, first_name, id;
