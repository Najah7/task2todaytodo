-- name: GetScheduleOccurrenceByActor :one
SELECT s.*
FROM schedules s
WHERE s.rule_id = sqlc.arg(rule_id)::text
  AND s.occurrence_date = sqlc.arg(occurrence_date)::date
  AND schedule_has_permission(s.id, sqlc.arg(actor_id)::text, 'schedule', 'read');

-- name: GetScheduleOccurrenceForHistoryByActor :one
SELECT s.*
FROM schedules s
JOIN schedule_rules b ON b.id = s.rule_id AND b.deleted_at IS NULL
WHERE s.rule_id = sqlc.arg(rule_id)::text
  AND s.occurrence_date = sqlc.arg(occurrence_date)::date
  AND (
      (s.deleted_at IS NOT NULL AND s.user_id = sqlc.arg(actor_id)::text)
      OR (s.deleted_at IS NULL AND schedule_has_permission(s.id, sqlc.arg(actor_id)::text, 'schedule', 'read'))
  );

-- name: GetScheduleOccurrenceForMutation :one
SELECT s.*
FROM schedules s
WHERE s.rule_id = sqlc.arg(rule_id)::text
  AND s.occurrence_date = sqlc.arg(occurrence_date)::date
  AND schedule_occurrence_has_permission(s.id, sqlc.arg(actor_id)::text, sqlc.arg(resource_id)::text, sqlc.arg(action)::action);

-- name: ListScheduleOccurrenceOriginsForRangeByActor :many
SELECT s.rule_id, s.occurrence_date
FROM schedules s
JOIN schedule_rule_revisions source ON source.id = s.source_revision_id
LEFT JOIN LATERAL (
    SELECT MAX(revision.local_end::date - revision.local_start::date) AS maximum_span
    FROM schedule_rule_revisions revision
    WHERE revision.rule_id = s.rule_id AND revision.superseded_at IS NULL
) spans ON true
WHERE s.occurrence_date BETWEEN
      sqlc.arg(from_at)::timestamptz::date - GREATEST(
          (source.local_end::date - source.local_start::date), COALESCE(spans.maximum_span, 0)
      ) - 2
      AND sqlc.arg(to_at)::timestamptz::date + 2
  AND s.rule_id = ANY(sqlc.arg(rule_ids)::text[])
ORDER BY s.rule_id, s.occurrence_date;

-- name: ListScheduleOccurrencesForRangeByActor :many
SELECT s.*
FROM schedules s
WHERE s.deleted_at IS NULL
  AND (sqlc.arg(include_skipped)::boolean OR s.skipped_at IS NULL)
  AND s.start_at < sqlc.arg(to_at)::timestamptz AND s.end_at > sqlc.arg(from_at)::timestamptz
  AND schedule_has_permission(s.id, sqlc.arg(actor_id)::text, 'schedule', 'read')
ORDER BY s.start_at, s.rule_id, s.occurrence_date;

-- name: ListScheduleOccurrenceCapabilities :many
WITH requested AS (
    SELECT ids.rule_id::text AS rule_id, dates.occurrence_date::date AS occurrence_date
    FROM unnest(sqlc.arg(rule_ids)::text[]) WITH ORDINALITY AS ids(rule_id, position)
    JOIN unnest(sqlc.arg(occurrence_dates)::text[]) WITH ORDINALITY AS dates(occurrence_date, position)
      USING (position)
)
SELECT requested.rule_id, requested.occurrence_date,
    CASE WHEN s.id IS NOT NULL THEN schedule_occurrence_has_permission(s.id, sqlc.arg(actor_id)::text, 'schedule', 'update')
         ELSE schedule_rule_has_permission_at(b.id, requested.occurrence_date, sqlc.arg(actor_id)::text, 'schedule', 'update') END::boolean AS can_update,
    CASE WHEN s.id IS NOT NULL THEN schedule_occurrence_has_permission(s.id, sqlc.arg(actor_id)::text, 'schedule', 'delete')
         ELSE schedule_rule_has_permission_at(b.id, requested.occurrence_date, sqlc.arg(actor_id)::text, 'schedule', 'delete') END::boolean AS can_delete,
    CASE WHEN s.id IS NOT NULL THEN schedule_occurrence_has_permission(s.id, sqlc.arg(actor_id)::text, 'occurrence', 'update')
         ELSE schedule_rule_has_permission_at(b.id, requested.occurrence_date, sqlc.arg(actor_id)::text, 'occurrence', 'update') END::boolean AS can_skip,
    CASE WHEN s.id IS NOT NULL THEN schedule_occurrence_has_permission(s.id, sqlc.arg(actor_id)::text, 'schedule_assignment', 'update')
         ELSE schedule_rule_has_permission_at(b.id, requested.occurrence_date, sqlc.arg(actor_id)::text, 'schedule_assignment', 'update') END::boolean AS can_assign,
    schedule_rule_has_permission_at(b.id, requested.occurrence_date, sqlc.arg(actor_id)::text, 'schedule', 'update') AS can_update_following,
    schedule_rule_has_permission_at(b.id, requested.occurrence_date, sqlc.arg(actor_id)::text, 'schedule', 'delete') AS can_delete_following
FROM requested
JOIN schedule_rules b ON b.id = requested.rule_id AND b.deleted_at IS NULL
LEFT JOIN schedules s ON s.rule_id = requested.rule_id AND s.occurrence_date = requested.occurrence_date
WHERE s.id IS NOT NULL OR schedule_rule_has_permission_at(b.id, requested.occurrence_date,
      sqlc.arg(actor_id)::text, 'schedule', 'read');

-- name: ListSchedulePauseIntervalsByActor :many
SELECT pauses.project_id, pauses.paused_at, pauses.resumed_at
FROM schedule_project_pause_intervals pauses
JOIN schedule_rules b ON b.id = pauses.rule_id
WHERE pauses.rule_id = sqlc.arg(rule_id)::text
ORDER BY pauses.paused_at;
