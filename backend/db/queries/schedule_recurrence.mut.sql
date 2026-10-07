-- name: SetScheduleRecurrenceByUserID :execrows
UPDATE schedules root
SET repeat_state = 'active',
    frequency_anchor_date = sqlc.arg(frequency_anchor_date)::date,
    interval_weeks = sqlc.arg(interval_weeks)::integer,
    changed_by = sqlc.arg(user_id)::text,
    updated_at = now()
WHERE root.id = sqlc.arg(series_id)::text
  AND root.id = root.series_id
  AND root.deleted_at IS NULL
  AND schedule_has_permission(root.id, sqlc.arg(user_id)::text, 'schedule', 'update')
  AND sqlc.arg(interval_weeks)::integer > 0;

-- name: ReplaceScheduleFrequenciesByUserID :exec
DELETE FROM schedule_frequencies f
USING schedules root
WHERE f.schedule_id = root.id
  AND root.id = sqlc.arg(series_id)::text
  AND root.id = root.series_id
  AND root.deleted_at IS NULL
  AND schedule_has_permission(root.id, sqlc.arg(user_id)::text, 'schedule', 'update');

-- name: CreateScheduleFrequenciesByUserID :exec
INSERT INTO schedule_frequencies (schedule_id, frequency)
SELECT root.id, f.frequency
FROM schedules root
CROSS JOIN unnest(sqlc.arg(frequencies)::text[]) AS f(frequency)
WHERE root.id = sqlc.arg(series_id)::text
  AND root.id = root.series_id
  AND root.deleted_at IS NULL
  AND schedule_has_permission(root.id, sqlc.arg(user_id)::text, 'schedule', 'update')
ON CONFLICT DO NOTHING;

-- name: StopScheduleRecurrenceByUserID :execrows
UPDATE schedules root
SET repeat_state = 'stopped', interval_weeks = 0, changed_by = sqlc.arg(user_id)::text, updated_at = now()
WHERE root.id = sqlc.arg(series_id)::text
  AND root.id = root.series_id
  AND root.deleted_at IS NULL
  AND schedule_has_permission(root.id, sqlc.arg(user_id)::text, 'schedule', 'update')
  AND root.repeat_state IN ('active', 'stopped');

-- name: ClearScheduleFrequenciesByUserID :exec
DELETE FROM schedule_frequencies f
USING schedules root
WHERE f.schedule_id = root.id
  AND root.id = sqlc.arg(series_id)::text
  AND root.id = root.series_id
  AND root.deleted_at IS NULL
  AND schedule_has_permission(root.id, sqlc.arg(user_id)::text, 'schedule', 'update');

-- name: SnapshotScheduleRootOccurrenceByUserID :exec
INSERT INTO schedules (
    id, user_id, project_id, assignee_id, title, description, location, start_at, end_at, completed,
    series_id, occurrence_date, timezone, is_exception, repeat_state,
    frequency_anchor_date, interval_weeks, deleted_at, changed_by
)
SELECT sqlc.arg(id)::text, root.user_id, root.project_id, root.assignee_id,
       root.title, root.description, root.location, root.start_at, root.end_at, root.completed,
       root.id, root.occurrence_date, root.timezone, true, NULL, NULL, 0, root.deleted_at, sqlc.arg(user_id)::text
FROM schedules root
WHERE root.id = sqlc.arg(series_id)::text
  AND root.id = root.series_id
  AND root.deleted_at IS NULL
  AND schedule_has_permission(root.id, sqlc.arg(user_id)::text, 'schedule', 'update')
ON CONFLICT (series_id, occurrence_date) WHERE id <> series_id DO NOTHING;

-- name: UpdateScheduleSeriesTemplateByUserID :execrows
UPDATE schedules root
SET title = sqlc.arg(title)::text,
    description = sqlc.narg(description)::text,
    location = sqlc.narg(location)::text,
    start_at = sqlc.arg(start_at)::timestamptz,
    end_at = sqlc.arg(end_at)::timestamptz,
    timezone = sqlc.arg(timezone)::text,
    changed_by = sqlc.arg(user_id)::text,
    updated_at = now()
WHERE root.id = sqlc.arg(series_id)::text
  AND root.id = root.series_id
  AND root.deleted_at IS NULL
  AND schedule_has_permission(root.id, sqlc.arg(user_id)::text, 'schedule', 'update');
