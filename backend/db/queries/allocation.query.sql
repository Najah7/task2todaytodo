-- name: GetAllocationSleepProfile :one
SELECT timezone, wake_up_time, bedtime FROM users WHERE id = sqlc.arg(user_id)::text;

-- name: ListAllocationUserIDs :many
SELECT id FROM users ORDER BY id;

-- name: ListProjectAllocationPlans :many
SELECT t.project_id, p.title, t.target_minutes
FROM project_time_allocation_plans t
JOIN projects p ON p.id = t.project_id
WHERE t.user_id = sqlc.arg(user_id)::text AND p.deleted_at IS NULL
  AND project_has_permission(p.id, sqlc.arg(user_id)::text, 'project', 'read'::action)
ORDER BY p.title, t.project_id;

-- name: ListAllocationProjects :many
SELECT p.id, p.title, t.target_minutes
FROM projects p
LEFT JOIN project_time_allocation_plans t
  ON t.project_id = p.id AND t.user_id = sqlc.arg(user_id)::text
WHERE p.deleted_at IS NULL AND project_has_permission(p.id, sqlc.arg(user_id)::text, 'project', 'read'::action)
ORDER BY p.title, p.id;

-- name: GetProjectAllocationProject :one
SELECT p.id, p.title, p.status FROM projects p
WHERE p.id = sqlc.arg(project_id)::text AND p.deleted_at IS NULL
  AND project_has_permission(p.id, sqlc.arg(user_id)::text, 'project', 'read'::action);

-- name: ListProjectAllocationRules :many
SELECT r.id AS rule_id, r.user_id, v.project_id, p.title AS project_title, r.pause_reason, r.paused_at, r.stopped_after,
       v.id AS source_revision_id, v.revision, v.effective_from, v.effective_at, v.weekday,
       v.local_start, v.local_end, v.end_day_offset, v.timezone, v.frequency
FROM project_time_allocation_rules r
JOIN LATERAL (
    SELECT revision.* FROM project_time_allocation_rule_revisions revision
    WHERE revision.rule_id = r.id AND revision.superseded_at IS NULL
    ORDER BY revision.effective_from DESC, revision.revision DESC LIMIT 1
) v ON true
JOIN projects p ON p.id = v.project_id
WHERE r.user_id = sqlc.arg(user_id)::text AND r.deleted_at IS NULL
  AND p.deleted_at IS NULL
  AND project_has_permission(v.project_id, sqlc.arg(user_id)::text, 'project', 'read'::action)
ORDER BY r.project_id, v.weekday, v.local_start, r.id;

-- name: ListAllocationProjectFrames :many
SELECT a.*, r.pause_reason,
       e.source_revision_id AS exception_source_revision_id,
       e.project_id AS exception_project_id,
       e.start_at AS exception_start_at, e.end_at AS exception_end_at,
       e.timezone AS exception_timezone, e.deleted_at AS exception_deleted_at
FROM project_time_allocations a
JOIN project_time_allocation_rules r ON r.id = a.rule_id
LEFT JOIN project_time_allocation_exceptions e
  ON e.rule_id = a.rule_id AND e.occurrence_date = a.occurrence_date
WHERE a.user_id = sqlc.arg(user_id)::text AND a.deleted_at IS NULL
  AND (e.rule_id IS NULL OR e.deleted_at IS NULL)
  AND (r.paused_at IS NULL OR COALESCE(e.start_at, a.start_at) < r.paused_at)
  AND NOT EXISTS (
      SELECT 1 FROM project_time_allocation_pause_intervals pause
      WHERE pause.rule_id = a.rule_id
        AND COALESCE(e.start_at, a.start_at) >= pause.paused_at
        AND (pause.resumed_at IS NULL OR COALESCE(e.start_at, a.start_at) < pause.resumed_at)
  )
  AND COALESCE(e.start_at, a.start_at) < sqlc.arg(through_at)::timestamptz
  AND COALESCE(e.end_at, a.end_at) > sqlc.arg(from_at)::timestamptz
  AND project_has_permission(COALESCE(e.project_id, a.project_id), sqlc.arg(user_id)::text, 'project', 'read'::action)
ORDER BY COALESCE(e.start_at, a.start_at), COALESCE(e.end_at, a.end_at), a.id;

-- name: FindAllocationOverlap :one
SELECT COALESCE(e.rule_id, a.rule_id) AS rule_id,
       COALESCE(e.occurrence_date, a.occurrence_date) AS occurrence_date,
       COALESCE(e.project_id, a.project_id) AS project_id,
       COALESCE(e.start_at, a.start_at) AS start_at,
       COALESCE(e.end_at, a.end_at) AS end_at
FROM project_time_allocations a
JOIN project_time_allocation_rules r ON r.id = a.rule_id
LEFT JOIN project_time_allocation_exceptions e
  ON e.rule_id = a.rule_id AND e.occurrence_date = a.occurrence_date
WHERE a.user_id = sqlc.arg(user_id)::text AND a.deleted_at IS NULL
  AND (e.rule_id IS NULL OR e.deleted_at IS NULL)
  AND COALESCE(e.start_at, a.start_at) < sqlc.arg(through_at)::timestamptz
  AND COALESCE(e.end_at, a.end_at) > sqlc.arg(from_at)::timestamptz
  AND (sqlc.narg(exclude_rule_id)::text IS NULL
       OR (a.rule_id, a.occurrence_date) IS DISTINCT FROM
          (sqlc.narg(exclude_rule_id)::text, sqlc.narg(exclude_occurrence_date)::date))
  AND (r.paused_at IS NULL OR COALESCE(e.start_at, a.start_at) < r.paused_at)
  AND NOT EXISTS (
      SELECT 1 FROM project_time_allocation_pause_intervals pause
      WHERE pause.rule_id = a.rule_id
        AND COALESCE(e.start_at, a.start_at) >= pause.paused_at
        AND (pause.resumed_at IS NULL OR COALESCE(e.start_at, a.start_at) < pause.resumed_at)
  )
  AND project_has_permission(COALESCE(e.project_id, a.project_id), sqlc.arg(user_id)::text, 'project', 'read'::action)
ORDER BY COALESCE(e.start_at, a.start_at) LIMIT 1;

-- name: GetAllocationOccurrenceForMutation :one
SELECT a.id, a.rule_id, a.source_revision_id, a.project_id, a.occurrence_date,
       COALESCE(e.start_at, a.start_at) AS start_at,
       COALESCE(e.end_at, a.end_at) AS end_at,
       COALESCE(e.timezone, a.timezone) AS timezone,
       a.revision, e.source_revision_id AS exception_source_revision_id,
       COALESCE(e.project_id, a.project_id) AS effective_project_id
FROM project_time_allocations a
JOIN project_time_allocation_rules r ON r.id = a.rule_id
LEFT JOIN project_time_allocation_exceptions e ON e.rule_id = a.rule_id AND e.occurrence_date = a.occurrence_date
WHERE a.rule_id = sqlc.arg(rule_id)::text AND a.user_id = sqlc.arg(user_id)::text
  AND a.occurrence_date = sqlc.arg(occurrence_date)::date AND a.deleted_at IS NULL
  AND (e.rule_id IS NULL OR e.deleted_at IS NULL)
  AND project_has_permission(COALESCE(e.project_id, a.project_id), sqlc.arg(user_id)::text, 'project', 'read'::action)
FOR UPDATE OF a;

-- name: ReadAllocationOccurrence :one
SELECT a.id, a.rule_id, a.source_revision_id, a.project_id, a.occurrence_date,
       COALESCE(e.start_at, a.start_at) AS start_at,
       COALESCE(e.end_at, a.end_at) AS end_at,
       COALESCE(e.timezone, a.timezone) AS timezone,
       a.revision, e.source_revision_id AS exception_source_revision_id,
       COALESCE(e.project_id, a.project_id) AS effective_project_id
FROM project_time_allocations a
JOIN project_time_allocation_rules r ON r.id = a.rule_id
LEFT JOIN project_time_allocation_exceptions e ON e.rule_id = a.rule_id AND e.occurrence_date = a.occurrence_date
WHERE a.rule_id = sqlc.arg(rule_id)::text AND a.user_id = sqlc.arg(user_id)::text
  AND a.occurrence_date = sqlc.arg(occurrence_date)::date AND a.deleted_at IS NULL
  AND (e.rule_id IS NULL OR e.deleted_at IS NULL)
  AND project_has_permission(COALESCE(e.project_id, a.project_id), sqlc.arg(user_id)::text, 'project', 'read'::action);

-- name: ListAllocationExceptionOrigins :many
SELECT e.rule_id, e.occurrence_date
FROM project_time_allocation_exceptions e
JOIN project_time_allocation_rules r ON r.id = e.rule_id
WHERE e.user_id = sqlc.arg(user_id)::text
  AND e.occurrence_date BETWEEN sqlc.arg(from_date)::date AND sqlc.arg(through_date)::date
ORDER BY e.rule_id, e.occurrence_date;

-- name: GetDailySleepException :one
SELECT * FROM daily_sleep_exceptions
WHERE user_id = sqlc.arg(user_id)::text AND local_date = sqlc.arg(local_date)::date;

-- name: ListDailySleepExceptions :many
SELECT * FROM daily_sleep_exceptions
WHERE user_id = sqlc.arg(user_id)::text
  AND local_date BETWEEN sqlc.arg(from_date)::date AND sqlc.arg(through_date)::date
ORDER BY local_date;

-- name: ListAllocationRulesForMaterialization :many
SELECT r.id AS rule_id, r.user_id, v.project_id, r.pause_reason, r.stopped_after,
       v.id AS source_revision_id, v.revision, v.effective_from, v.effective_at, v.weekday,
       v.local_start, v.local_end, v.end_day_offset, v.timezone, v.frequency
FROM project_time_allocation_rules r
JOIN project_time_allocation_rule_revisions v ON v.rule_id = r.id AND v.superseded_at IS NULL
JOIN projects p ON p.id = v.project_id AND p.deleted_at IS NULL
WHERE r.user_id = sqlc.arg(user_id)::text AND r.deleted_at IS NULL
  AND r.pause_reason IS NULL AND v.effective_from <= sqlc.arg(through_date)::date
  AND project_has_permission(v.project_id, r.user_id, 'project', 'read'::action)
ORDER BY r.id;

-- name: ListAllocationPauseIntervals :many
SELECT pause.*
FROM project_time_allocation_pause_intervals pause
WHERE pause.rule_id = sqlc.arg(rule_id)::text
ORDER BY pause.paused_at;

