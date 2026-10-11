-- name: GetScheduleOccurrenceByActor :one
SELECT s.*
FROM schedules s
WHERE s.blueprint_id = sqlc.arg(blueprint_id)::text
  AND s.occurrence_date = sqlc.arg(occurrence_date)::date
  AND schedule_has_permission(s.id, sqlc.arg(actor_id)::text, 'schedule', 'read');

-- name: GetScheduleOccurrenceForHistoryByActor :one
SELECT s.*
FROM schedules s
JOIN schedule_blueprints b ON b.id = s.blueprint_id AND b.deleted_at IS NULL
WHERE s.blueprint_id = sqlc.arg(blueprint_id)::text
  AND s.occurrence_date = sqlc.arg(occurrence_date)::date
  AND (
      (s.deleted_at IS NOT NULL AND s.user_id = sqlc.arg(actor_id)::text)
      OR (s.deleted_at IS NULL AND schedule_has_permission(s.id, sqlc.arg(actor_id)::text, 'schedule', 'read'))
  );

-- name: GetScheduleOccurrenceForMutation :one
SELECT s.*
FROM schedules s
WHERE s.blueprint_id = sqlc.arg(blueprint_id)::text
  AND s.occurrence_date = sqlc.arg(occurrence_date)::date
  AND schedule_occurrence_has_permission(s.id, sqlc.arg(actor_id)::text, sqlc.arg(resource_id)::text, sqlc.arg(action)::action);

-- name: ListScheduleOccurrenceOriginsForRangeByActor :many
SELECT s.blueprint_id, s.occurrence_date
FROM schedules s
JOIN schedule_blueprint_revisions source ON source.id = s.source_revision_id
LEFT JOIN LATERAL (
    SELECT MAX(revision.local_end::date - revision.local_start::date) AS maximum_span
    FROM schedule_blueprint_revisions revision
    WHERE revision.blueprint_id = s.blueprint_id AND revision.superseded_at IS NULL
) spans ON true
WHERE s.occurrence_date BETWEEN
      sqlc.arg(from_at)::timestamptz::date - GREATEST(
          (source.local_end::date - source.local_start::date), COALESCE(spans.maximum_span, 0)
      ) - 2
      AND sqlc.arg(to_at)::timestamptz::date + 2
  AND s.blueprint_id = ANY(sqlc.arg(blueprint_ids)::text[])
ORDER BY s.blueprint_id, s.occurrence_date;

-- name: ListScheduleOccurrencesForRangeByActor :many
SELECT s.*
FROM schedules s
WHERE s.deleted_at IS NULL
  AND s.start_at < sqlc.arg(to_at)::timestamptz AND s.end_at > sqlc.arg(from_at)::timestamptz
  AND schedule_has_permission(s.id, sqlc.arg(actor_id)::text, 'schedule', 'read')
ORDER BY s.start_at, s.blueprint_id, s.occurrence_date;

-- name: ListSchedulePauseIntervalsByActor :many
SELECT pauses.project_id, pauses.paused_at, pauses.resumed_at
FROM schedule_project_pause_intervals pauses
JOIN schedule_blueprints b ON b.id = pauses.blueprint_id
WHERE pauses.blueprint_id = sqlc.arg(blueprint_id)::text
ORDER BY pauses.paused_at;
