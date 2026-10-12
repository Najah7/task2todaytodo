-- name: ListProjectAllocationRuleRevisionsForProjection :many
WITH active_revisions AS (
    SELECT revision.*,
           LEAD(revision.effective_from) OVER (
               PARTITION BY revision.rule_id
               ORDER BY revision.effective_from, revision.revision
           ) AS next_effective_from,
           LEAD(revision.effective_at) OVER (
               PARTITION BY revision.rule_id
               ORDER BY revision.effective_from, revision.revision
           ) AS next_effective_at,
           LEAD(revision.timezone) OVER (
               PARTITION BY revision.rule_id
               ORDER BY revision.effective_from, revision.revision
           ) AS next_timezone,
           MAX(revision.revision) OVER (PARTITION BY revision.rule_id)::integer AS current_rule_revision
    FROM project_time_allocation_rule_revisions revision
    WHERE revision.superseded_at IS NULL
)
SELECT revision.rule_id, revision.project_id, revision.id AS source_revision_id,
       revision.revision, revision.current_rule_revision,
       revision.effective_from, revision.recurrence_start_date, revision.effective_at,
       CASE WHEN revision.next_effective_from IS NULL THEN NULL
            ELSE revision.next_effective_from END::date AS effective_until,
       CASE WHEN revision.next_effective_from IS NULL THEN NULL
            ELSE GREATEST(
                revision.next_effective_at,
                revision.next_effective_from::timestamp AT TIME ZONE revision.next_timezone
            ) END::timestamptz AS effective_until_at,
       revision.weekday, revision.local_start, revision.local_end,
       revision.end_day_offset, revision.timezone, revision.frequency,
       rule.pause_reason, rule.paused_at, rule.stopped_after, rule.deleted_at
FROM active_revisions revision
JOIN project_time_allocation_rules rule ON rule.id = revision.rule_id
WHERE rule.user_id = sqlc.arg(user_id)::text
  AND (revision.project_id IS NULL OR project_has_permission(revision.project_id, rule.user_id, 'project', 'read'::action))
ORDER BY revision.rule_id, revision.effective_from, revision.revision;
