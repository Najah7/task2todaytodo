-- name: ListProjectAllocationPauseRulesForResume :many
SELECT pause.rule_id, rule.user_id
FROM project_time_allocation_pause_intervals pause
JOIN project_time_allocation_rules rule ON rule.id = pause.rule_id
WHERE pause.project_id = sqlc.arg(project_id)::text
  AND pause.resumed_at IS NULL
  AND rule.deleted_at IS NULL
  AND rule.pause_reason = 'project_completed'
ORDER BY rule.user_id, pause.rule_id;

-- name: StopProjectAllocationRule :exec
UPDATE project_time_allocation_rules
SET deleted_at = sqlc.arg(cutoff)::timestamptz
WHERE id = sqlc.arg(rule_id)::text
  AND user_id = sqlc.arg(user_id)::text
  AND (deleted_at IS NULL OR deleted_at > sqlc.arg(cutoff)::timestamptz);

-- name: DeleteFollowingProjectAllocationRowsByOrigin :exec
UPDATE project_time_allocations
SET deleted_at = now(), revision = revision + 1, changed_by = sqlc.arg(changed_by)::text
WHERE rule_id = sqlc.arg(rule_id)::text
  AND occurrence_date >= sqlc.arg(origin_date)::date
  AND COALESCE((
      SELECT exception.start_at FROM project_time_allocation_exceptions exception
      WHERE exception.rule_id = project_time_allocations.rule_id
        AND exception.occurrence_date = project_time_allocations.occurrence_date
        AND exception.deleted_at IS NULL
  ), project_time_allocations.start_at) >= sqlc.arg(cutoff)::timestamptz
  AND deleted_at IS NULL;

-- name: DeleteFollowingProjectAllocationExceptionsByOrigin :exec
UPDATE project_time_allocation_exceptions
SET deleted_at = now(), changed_by = sqlc.arg(changed_by)::text, updated_at = now()
WHERE rule_id = sqlc.arg(rule_id)::text
  AND occurrence_date >= sqlc.arg(origin_date)::date
  AND start_at >= sqlc.arg(cutoff)::timestamptz
  AND deleted_at IS NULL;

-- name: StopFollowingProjectAllocationRule :exec
UPDATE project_time_allocation_rules
SET stopped_after = sqlc.arg(stopped_after)::date,
    deleted_at = COALESCE(deleted_at, sqlc.arg(cutoff)::timestamptz)
WHERE id = sqlc.arg(rule_id)::text
  AND user_id = sqlc.arg(user_id)::text
  AND (stopped_after IS NULL OR stopped_after > sqlc.arg(stopped_after)::date);

-- name: DeleteFutureOrdinaryAllocationOriginsAfter :exec
UPDATE project_time_allocations allocation
SET deleted_at = now(), revision = revision + 1, changed_by = sqlc.arg(changed_by)::text
WHERE allocation.rule_id = sqlc.arg(rule_id)::text
  AND allocation.occurrence_date > sqlc.arg(origin_date)::date
  AND allocation.start_at >= sqlc.arg(cutoff)::timestamptz
  AND allocation.deleted_at IS NULL
  AND NOT EXISTS (
      SELECT 1 FROM project_time_allocation_exceptions exception
      WHERE exception.rule_id = allocation.rule_id
        AND exception.occurrence_date = allocation.occurrence_date
  );

-- name: StartAllocationPauseInterval :exec
INSERT INTO project_time_allocation_pause_intervals(project_id, rule_id, paused_at)
VALUES (sqlc.arg(project_id)::text, sqlc.arg(rule_id)::text, sqlc.arg(paused_at)::timestamptz)
ON CONFLICT (project_id, rule_id, paused_at) DO NOTHING;

-- name: PauseProjectAllocationRulesForLifecycle :many
UPDATE project_time_allocation_rules rule
SET pause_reason = 'project_completed', paused_at = COALESCE(rule.paused_at, sqlc.arg(paused_at)::timestamptz)
WHERE rule.id IN (
    SELECT current_rule.id
    FROM project_time_allocation_rules current_rule
    JOIN LATERAL (
        SELECT revision.project_id FROM project_time_allocation_rule_revisions revision
        WHERE revision.rule_id = current_rule.id AND revision.superseded_at IS NULL
        ORDER BY revision.effective_from DESC, revision.revision DESC LIMIT 1
    ) current_revision ON true
    WHERE current_revision.project_id = sqlc.arg(project_id)::text
      AND current_rule.deleted_at IS NULL AND current_rule.pause_reason IS NULL
)
RETURNING rule.user_id;

-- name: StartProjectAllocationPauseIntervalsForLifecycle :exec
INSERT INTO project_time_allocation_pause_intervals(project_id, rule_id, paused_at)
SELECT current_revision.project_id, rule.id, sqlc.arg(paused_at)::timestamptz
FROM project_time_allocation_rules rule
JOIN LATERAL (
    SELECT revision.project_id FROM project_time_allocation_rule_revisions revision
    WHERE revision.rule_id = rule.id AND revision.superseded_at IS NULL
    ORDER BY revision.effective_from DESC, revision.revision DESC LIMIT 1
) current_revision ON true
WHERE current_revision.project_id = sqlc.arg(project_id)::text
  AND rule.deleted_at IS NULL AND rule.pause_reason = 'project_completed'
  AND rule.paused_at = sqlc.arg(paused_at)::timestamptz
ON CONFLICT (project_id, rule_id, paused_at) DO NOTHING;

-- name: ResumeProjectAllocationRule :exec
UPDATE project_time_allocation_rules
SET pause_reason = NULL, paused_at = NULL
WHERE id = sqlc.arg(rule_id)::text
  AND user_id = sqlc.arg(user_id)::text
  AND pause_reason = 'project_completed';

-- name: CloseProjectAllocationPauseInterval :exec
UPDATE project_time_allocation_pause_intervals
SET resumed_at = sqlc.arg(resumed_at)::timestamptz
WHERE project_id = sqlc.arg(project_id)::text
  AND rule_id = sqlc.arg(rule_id)::text
  AND resumed_at IS NULL;
