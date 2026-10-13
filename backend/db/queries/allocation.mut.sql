-- name: LockAllocationUser :exec
SELECT pg_advisory_xact_lock(hashtext('allocation/user'), hashtext(sqlc.arg(user_id)::text));

-- name: SetProjectAllocationPlan :exec
INSERT INTO project_time_allocation_plans(user_id, project_id, target_minutes, updated_at)
VALUES (sqlc.arg(user_id)::text, sqlc.arg(project_id)::text, sqlc.arg(target_minutes)::integer, now())
ON CONFLICT (user_id, project_id) DO UPDATE
SET target_minutes = EXCLUDED.target_minutes, updated_at = now();

-- name: DeleteProjectAllocationPlan :exec
DELETE FROM project_time_allocation_plans
WHERE user_id = sqlc.arg(user_id)::text AND project_id = sqlc.arg(project_id)::text;

-- name: CreateProjectTimeAllocationRule :one
INSERT INTO project_time_allocation_rules(id, user_id, project_id)
VALUES (sqlc.arg(id)::text, sqlc.arg(user_id)::text, sqlc.arg(project_id)::text)
RETURNING *;

-- name: CreateProjectTimeAllocationRuleRevision :one
INSERT INTO project_time_allocation_rule_revisions(
    id, rule_id, user_id, project_id, revision, effective_from, recurrence_start_date, effective_at, weekday, local_start, local_end,
    end_day_offset, timezone, frequency, changed_by
)
VALUES (
    sqlc.arg(id)::text, sqlc.arg(rule_id)::text, sqlc.arg(user_id)::text,
    sqlc.arg(project_id)::text, sqlc.arg(revision)::integer, sqlc.arg(effective_from)::date, sqlc.arg(recurrence_start_date)::date, sqlc.arg(effective_at)::timestamptz, sqlc.arg(weekday)::smallint,
    sqlc.arg(local_start)::time, sqlc.arg(local_end)::time,
    sqlc.arg(end_day_offset)::smallint, sqlc.arg(timezone)::text,
    sqlc.arg(frequency)::text, sqlc.arg(changed_by)::text
)
RETURNING *;

-- name: SupersedeFutureAllocationRevisions :exec
UPDATE project_time_allocation_rule_revisions
SET superseded_at = now()
WHERE rule_id = sqlc.arg(rule_id)::text AND superseded_at IS NULL
  AND effective_at >= sqlc.arg(effective_at)::timestamptz;

-- name: DeleteFutureProjectAllocations :exec
UPDATE project_time_allocations
SET deleted_at = now(), revision = revision + 1, changed_by = sqlc.arg(changed_by)::text
WHERE rule_id = sqlc.arg(rule_id)::text AND start_at >= sqlc.arg(cutoff)::timestamptz
  AND deleted_at IS NULL
  AND NOT EXISTS (
      SELECT 1 FROM project_time_allocation_exceptions e
      WHERE e.rule_id = project_time_allocations.rule_id
        AND e.occurrence_date = project_time_allocations.occurrence_date
  );

-- name: StopProjectAllocationRuleAfter :exec
UPDATE project_time_allocation_rules
SET stopped_after = sqlc.arg(occurrence_date)::date
WHERE id = sqlc.arg(rule_id)::text AND user_id = sqlc.arg(user_id)::text
  AND (stopped_after IS NULL OR stopped_after > sqlc.arg(occurrence_date)::date);

-- name: DeleteFutureProjectAllocationRuleRows :exec
UPDATE project_time_allocations
SET deleted_at = now(), revision = revision + 1, changed_by = sqlc.arg(changed_by)::text
WHERE rule_id = sqlc.arg(rule_id)::text AND start_at >= sqlc.arg(cutoff)::timestamptz AND deleted_at IS NULL;

-- name: DeleteFutureProjectAllocationExceptions :exec
UPDATE project_time_allocation_exceptions
SET deleted_at = now(), changed_by = sqlc.arg(changed_by)::text, updated_at = now()
WHERE rule_id = sqlc.arg(rule_id)::text AND start_at >= sqlc.arg(cutoff)::timestamptz AND deleted_at IS NULL;

-- name: InsertProjectTimeAllocation :one
INSERT INTO project_time_allocations(
    id, rule_id, source_revision_id, user_id, project_id, occurrence_date,
    start_at, end_at, timezone, changed_by
)
VALUES (
    sqlc.arg(id)::text, sqlc.arg(rule_id)::text, sqlc.arg(source_revision_id)::text,
    sqlc.arg(user_id)::text, sqlc.arg(project_id)::text, sqlc.arg(occurrence_date)::date,
    sqlc.arg(start_at)::timestamptz, sqlc.arg(end_at)::timestamptz,
    sqlc.arg(timezone)::text, sqlc.arg(changed_by)::text
)
ON CONFLICT (rule_id, occurrence_date) DO NOTHING
RETURNING *;

-- name: UpsertProjectTimeAllocationProjection :one
INSERT INTO project_time_allocations(
    id, rule_id, source_revision_id, user_id, project_id, occurrence_date,
    start_at, end_at, timezone, changed_by
)
VALUES (
    sqlc.arg(id)::text, sqlc.arg(rule_id)::text, sqlc.arg(source_revision_id)::text,
    sqlc.arg(user_id)::text, sqlc.arg(project_id)::text, sqlc.arg(occurrence_date)::date,
    sqlc.arg(start_at)::timestamptz, sqlc.arg(end_at)::timestamptz,
    sqlc.arg(timezone)::text, sqlc.arg(changed_by)::text
)
ON CONFLICT (rule_id, occurrence_date) DO UPDATE SET
    source_revision_id = EXCLUDED.source_revision_id,
    project_id = EXCLUDED.project_id,
    start_at = EXCLUDED.start_at,
    end_at = EXCLUDED.end_at,
    timezone = EXCLUDED.timezone,
    deleted_at = NULL,
    changed_by = EXCLUDED.changed_by,
    updated_at = now()
WHERE (
    (project_time_allocations.deleted_at IS NOT NULL AND EXCLUDED.start_at >= sqlc.arg(cutoff)::timestamptz)
    OR (project_time_allocations.deleted_at IS NULL AND project_time_allocations.start_at >= sqlc.arg(cutoff)::timestamptz)
   ) AND NOT EXISTS (
       SELECT 1 FROM project_time_allocation_exceptions e
       WHERE e.rule_id = project_time_allocations.rule_id
         AND e.occurrence_date = project_time_allocations.occurrence_date
   )
RETURNING *;

-- name: UpsertProjectTimeAllocationException :exec
INSERT INTO project_time_allocation_exceptions(
    rule_id, occurrence_date, source_revision_id, user_id, project_id,
    start_at, end_at, timezone, deleted_at, changed_by
)
VALUES (
    sqlc.arg(rule_id)::text, sqlc.arg(occurrence_date)::date,
    sqlc.arg(source_revision_id)::text, sqlc.arg(user_id)::text,
    sqlc.arg(project_id)::text, sqlc.arg(start_at)::timestamptz,
    sqlc.arg(end_at)::timestamptz, sqlc.arg(timezone)::text,
    sqlc.narg(deleted_at)::timestamptz, sqlc.arg(changed_by)::text
)
ON CONFLICT (rule_id, occurrence_date) DO UPDATE SET
    source_revision_id = EXCLUDED.source_revision_id,
    project_id = EXCLUDED.project_id,
    start_at = EXCLUDED.start_at,
    end_at = EXCLUDED.end_at,
    timezone = EXCLUDED.timezone,
    deleted_at = EXCLUDED.deleted_at,
    changed_by = EXCLUDED.changed_by,
    updated_at = now();

-- name: UpdateProjectTimeAllocationProjectionFromException :execrows
UPDATE project_time_allocations
SET source_revision_id = sqlc.arg(source_revision_id)::text,
    project_id = sqlc.arg(project_id)::text,
    start_at = sqlc.arg(start_at)::timestamptz,
    end_at = sqlc.arg(end_at)::timestamptz,
    timezone = sqlc.arg(timezone)::text,
    deleted_at = sqlc.narg(deleted_at)::timestamptz,
    changed_by = sqlc.arg(changed_by)::text,
    updated_at = now()
WHERE rule_id = sqlc.arg(rule_id)::text AND occurrence_date = sqlc.arg(occurrence_date)::date
  AND revision = sqlc.arg(expected_revision)::integer AND deleted_at IS NULL;

-- name: SetAllocationPause :exec
UPDATE project_time_allocation_rules
SET pause_reason = sqlc.narg(pause_reason)::text,
    paused_at = CASE WHEN sqlc.narg(pause_reason)::text IS NULL THEN NULL ELSE COALESCE(paused_at, sqlc.arg(paused_at)::timestamptz) END
WHERE id = sqlc.arg(rule_id)::text AND user_id = sqlc.arg(user_id)::text;

-- name: PauseProjectAllocationRules :exec
UPDATE project_time_allocation_rules
SET pause_reason = 'project_completed', paused_at = COALESCE(paused_at, sqlc.arg(paused_at)::timestamptz)
WHERE id IN (
    SELECT r.id FROM project_time_allocation_rules r
    JOIN LATERAL (
        SELECT revision.project_id FROM project_time_allocation_rule_revisions revision
        WHERE revision.rule_id = r.id AND revision.superseded_at IS NULL
        ORDER BY revision.effective_from DESC, revision.revision DESC LIMIT 1
    ) current_revision ON true
    WHERE current_revision.project_id = sqlc.arg(project_id)::text
      AND r.deleted_at IS NULL AND r.pause_reason IS NULL
);

-- name: StartProjectAllocationPauseIntervals :exec
INSERT INTO project_time_allocation_pause_intervals(project_id, rule_id, paused_at)
SELECT current_revision.project_id, r.id, sqlc.arg(paused_at)::timestamptz
FROM project_time_allocation_rules r
JOIN LATERAL (
    SELECT revision.project_id FROM project_time_allocation_rule_revisions revision
    WHERE revision.rule_id = r.id AND revision.superseded_at IS NULL
    ORDER BY revision.effective_from DESC, revision.revision DESC LIMIT 1
) current_revision ON true
WHERE current_revision.project_id = sqlc.arg(project_id)::text
  AND r.deleted_at IS NULL AND r.pause_reason = 'project_completed'
  AND r.paused_at = sqlc.arg(paused_at)::timestamptz
ON CONFLICT (project_id, rule_id, paused_at) DO NOTHING;

-- name: ResumeProjectAllocationRules :exec
UPDATE project_time_allocation_rules
SET pause_reason = NULL, paused_at = NULL
WHERE id IN (
    SELECT pause.rule_id FROM project_time_allocation_pause_intervals pause
    WHERE pause.project_id = sqlc.arg(project_id)::text AND pause.resumed_at IS NULL
) AND pause_reason = 'project_completed';

-- name: ResumeProjectAllocationPauseIntervals :exec
UPDATE project_time_allocation_pause_intervals
SET resumed_at = sqlc.arg(resumed_at)::timestamptz
WHERE project_id = sqlc.arg(project_id)::text AND resumed_at IS NULL;

-- name: CreateDailyActiveTimeException :one
INSERT INTO daily_active_time_exceptions(user_id, local_date, active_start_local, active_end_local, timezone)
VALUES (
    sqlc.arg(user_id)::text, sqlc.arg(local_date)::date,
    sqlc.arg(active_start_local)::timestamp, sqlc.arg(active_end_local)::timestamp,
    sqlc.arg(timezone)::text
)
ON CONFLICT (user_id, local_date) DO UPDATE SET
    active_start_local = EXCLUDED.active_start_local, active_end_local = EXCLUDED.active_end_local,
    timezone = EXCLUDED.timezone,
    revision = daily_active_time_exceptions.revision + 1, changed_at = now()
RETURNING *;

-- name: DeleteDailyActiveTimeException :execrows
DELETE FROM daily_active_time_exceptions
WHERE user_id = sqlc.arg(user_id)::text AND local_date = sqlc.arg(local_date)::date
  AND revision = sqlc.arg(revision)::integer;
