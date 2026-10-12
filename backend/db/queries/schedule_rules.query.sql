-- name: GetEffectiveScheduleRuleRevisionForDate :one
WITH active_revisions AS (
    SELECT r.*, LEAD(r.effective_from) OVER (PARTITION BY r.rule_id ORDER BY r.effective_from, r.revision) AS next_effective_from
    FROM schedule_rule_revisions r
    WHERE r.rule_id = sqlc.arg(rule_id)::text AND r.superseded_at IS NULL
), effective AS (
    SELECT r.*,
           CASE WHEN r.cutoff_after IS NULL THEN r.next_effective_from - 1
                WHEN r.next_effective_from IS NULL THEN r.cutoff_after
                ELSE LEAST(r.cutoff_after, r.next_effective_from - 1) END AS effective_until
    FROM active_revisions r
    WHERE r.effective_from <= sqlc.arg(occurrence_date)::date
    ORDER BY r.effective_from DESC, r.revision DESC LIMIT 1
)
SELECT effective.*
FROM effective
JOIN schedule_rules b ON b.id = effective.rule_id AND b.deleted_at IS NULL
WHERE schedule_rule_has_permission_at(b.id, sqlc.arg(occurrence_date)::date,
    sqlc.arg(actor_id)::text, sqlc.arg(resource_id)::text, sqlc.arg(action)::action)
  AND (effective.effective_until IS NULL OR effective.effective_until >= sqlc.arg(occurrence_date)::date);

-- name: GetScheduleRuleByActor :one
SELECT b.id, b.user_id, b.deleted_at IS NOT NULL AS deleted, b.created_at,
       r.id AS revision_id, (SELECT MAX(all_revisions.revision)::integer FROM schedule_rule_revisions all_revisions WHERE all_revisions.rule_id = b.id) AS revision, r.effective_from, r.recurrence_anchor_date,
       r.local_start, r.local_end, r.timezone, r.frequency, r.interval_count,
       r.all_day,
       r.weekdays, r.end_date, r.cutoff_after, r.title, r.description, r.location,
       r.project_id, r.assignee_id, r.changed_by, r.created_at AS revision_created_at
FROM schedule_rules b
JOIN LATERAL (
    SELECT revision.* FROM schedule_rule_revisions revision
    WHERE revision.rule_id = b.id
    ORDER BY
      CASE WHEN revision.superseded_at IS NULL THEN 0 ELSE 1 END,
      CASE WHEN revision.effective_from <= (now() AT TIME ZONE revision.timezone)::date THEN 0 ELSE 1 END,
      CASE WHEN revision.effective_from <= (now() AT TIME ZONE revision.timezone)::date THEN revision.effective_from END DESC,
      CASE WHEN revision.effective_from > (now() AT TIME ZONE revision.timezone)::date THEN revision.effective_from END,
      revision.revision DESC LIMIT 1
) r ON true
WHERE b.id = sqlc.arg(rule_id)::text
  AND schedule_rule_has_permission(b.id, sqlc.arg(actor_id)::text, 'schedule', 'read');

-- name: GetScheduleRuleRevisionForMutation :one
SELECT (SELECT COALESCE(MAX(revision.revision), 0)::integer
        FROM schedule_rule_revisions revision WHERE revision.rule_id = rule.id) AS revision
FROM schedule_rules rule
WHERE rule.id = sqlc.arg(rule_id)::text
  AND rule.deleted_at IS NULL
  AND schedule_rule_has_permission_at(rule.id, sqlc.arg(occurrence_date)::date,
      sqlc.arg(actor_id)::text, sqlc.arg(resource_id)::text, sqlc.arg(action)::action);

-- name: IsScheduleRuleAssigneeEligible :one
SELECT (CASE
  WHEN sqlc.narg(project_id)::text IS NULL THEN sqlc.arg(assignee_id)::text = sqlc.arg(owner_id)::text
  ELSE EXISTS (
    SELECT 1 FROM projects p
    WHERE p.id = sqlc.narg(project_id)::text AND p.user_id = sqlc.arg(owner_id)::text
      AND p.deleted_at IS NULL
      AND (p.user_id = sqlc.arg(assignee_id)::text OR EXISTS (
        SELECT 1 FROM project_members pm WHERE pm.project_id = p.id AND pm.user_id = sqlc.arg(assignee_id)::text
      ))
  )
END)::boolean;

-- name: ListScheduleTodoImpactUsers :many
WITH existing_assignees AS (
    SELECT assignee_id FROM schedules
    WHERE rule_id = sqlc.arg(rule_id)::text AND (
        sqlc.narg(occurrence_date)::date IS NULL OR
        (sqlc.arg(following)::boolean AND occurrence_date >= sqlc.narg(occurrence_date)::date) OR
        (NOT sqlc.arg(following)::boolean AND occurrence_date = sqlc.narg(occurrence_date)::date)
    )
    UNION ALL
    SELECT assignee_id FROM schedule_exceptions
    WHERE rule_id = sqlc.arg(rule_id)::text AND (
        sqlc.narg(occurrence_date)::date IS NULL OR
        (sqlc.arg(following)::boolean AND occurrence_date >= sqlc.narg(occurrence_date)::date) OR
        (NOT sqlc.arg(following)::boolean AND occurrence_date = sqlc.narg(occurrence_date)::date)
    )
    UNION ALL
    SELECT assignee_id FROM schedule_rule_revisions
    WHERE rule_id = sqlc.arg(rule_id)::text AND (
        sqlc.narg(occurrence_date)::date IS NULL OR
        (sqlc.arg(following)::boolean AND (effective_from >= sqlc.narg(occurrence_date)::date OR
            (effective_from <= sqlc.narg(occurrence_date)::date AND (cutoff_after IS NULL OR cutoff_after >= sqlc.narg(occurrence_date)::date)))) OR
        (NOT sqlc.arg(following)::boolean AND effective_from <= sqlc.narg(occurrence_date)::date AND (cutoff_after IS NULL OR cutoff_after >= sqlc.narg(occurrence_date)::date))
    )
), proposed_assignee AS (
    SELECT COALESCE(NULLIF(sqlc.narg(assignee_id)::text, ''), project.user_id, sqlc.arg(actor_id)::text) AS assignee_id
    FROM (SELECT 1) AS seed
    LEFT JOIN projects AS project ON project.id = sqlc.narg(project_id)::text AND project.deleted_at IS NULL
)
SELECT DISTINCT assignee_id
FROM (SELECT assignee_id FROM existing_assignees UNION ALL SELECT assignee_id FROM proposed_assignee) users
WHERE assignee_id <> ''
ORDER BY assignee_id;

-- name: ListEligibleScheduleRuleAssignees :many
WITH current_revision AS (
    SELECT revision.* FROM schedule_rule_revisions revision
    WHERE revision.rule_id = sqlc.arg(rule_id)::text
      AND revision.superseded_at IS NULL
      AND (revision.cutoff_after IS NULL OR revision.cutoff_after >= revision.effective_from)
    ORDER BY
      CASE WHEN revision.effective_from <= (now() AT TIME ZONE revision.timezone)::date THEN 0 ELSE 1 END,
      CASE WHEN revision.effective_from <= (now() AT TIME ZONE revision.timezone)::date THEN revision.effective_from END DESC,
      CASE WHEN revision.effective_from > (now() AT TIME ZONE revision.timezone)::date THEN revision.effective_from END,
      revision.revision DESC LIMIT 1
)
SELECT u.id, u.first_name, u.last_name, u.email, true AS is_owner
FROM schedule_rules b
JOIN users u ON u.id = b.user_id
CROSS JOIN current_revision current
WHERE b.id = current.rule_id
  AND schedule_rule_has_permission_at(b.id, current.effective_from, sqlc.arg(actor_id)::text, 'schedule_assignment', 'update')
UNION
SELECT u.id, u.first_name, u.last_name, u.email, false AS is_owner
FROM current_revision current
JOIN project_members pm ON pm.project_id = current.project_id
JOIN users u ON u.id = pm.user_id
WHERE current.project_id IS NOT NULL
  AND schedule_rule_has_permission_at(current.rule_id, current.effective_from, sqlc.arg(actor_id)::text, 'schedule_assignment', 'update')
ORDER BY is_owner DESC, last_name, first_name, id;

-- name: ListScheduleRuleRevisionsByActor :many
WITH active_boundaries AS (
    SELECT id,
           LEAD(effective_from) OVER (PARTITION BY rule_id ORDER BY effective_from, revision) AS next_effective_from
    FROM schedule_rule_revisions
    WHERE rule_id = sqlc.arg(rule_id)::text AND superseded_at IS NULL
)
SELECT r.*, b.deleted_at IS NOT NULL AS rule_deleted,
       CASE WHEN r.superseded_at IS NOT NULL THEN NULL
            WHEN r.cutoff_after IS NULL THEN active.next_effective_from - 1
            WHEN active.next_effective_from IS NULL THEN r.cutoff_after
            ELSE LEAST(r.cutoff_after, active.next_effective_from - 1) END AS effective_until
FROM schedule_rule_revisions r
JOIN schedule_rules b ON b.id = r.rule_id
LEFT JOIN active_boundaries active ON active.id = r.id
WHERE r.rule_id = sqlc.arg(rule_id)::text
  AND schedule_rule_has_permission(b.id, sqlc.arg(actor_id)::text, 'schedule', 'read')
  AND (sqlc.narg(before_revision)::integer IS NULL OR r.revision < sqlc.narg(before_revision)::integer)
ORDER BY r.revision DESC
LIMIT sqlc.arg(page_limit)::integer;

-- name: ListScheduleRuleRevisionsForBackfill :many
WITH active_revisions AS (
    SELECT r.*, LEAD(r.effective_from) OVER (PARTITION BY r.rule_id ORDER BY r.effective_from, r.revision) AS next_effective_from
    FROM schedule_rule_revisions r
    JOIN schedule_rules b ON b.id = r.rule_id AND b.deleted_at IS NULL
    WHERE r.superseded_at IS NULL
)
SELECT r.*, CASE WHEN r.cutoff_after IS NULL THEN r.next_effective_from - 1
                 WHEN r.next_effective_from IS NULL THEN r.cutoff_after
                 ELSE LEAST(r.cutoff_after, r.next_effective_from - 1) END AS effective_until
FROM active_revisions r
WHERE r.frequency <> 'once'
  AND (r.cutoff_after IS NULL OR r.cutoff_after >= r.effective_from)
  AND (r.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = r.project_id AND p.deleted_at IS NULL))
ORDER BY r.rule_id, r.effective_from, r.revision;

-- name: ListScheduleRuleRevisionsForBackfillByRule :many
WITH active_revisions AS (
    SELECT r.*, LEAD(r.effective_from) OVER (PARTITION BY r.rule_id ORDER BY r.effective_from, r.revision) AS next_effective_from
    FROM schedule_rule_revisions r
    JOIN schedule_rules b ON b.id = r.rule_id AND b.deleted_at IS NULL
    WHERE r.rule_id = sqlc.arg(rule_id)::text AND r.superseded_at IS NULL
)
SELECT r.*, CASE WHEN r.cutoff_after IS NULL THEN r.next_effective_from - 1
                 WHEN r.next_effective_from IS NULL THEN r.cutoff_after
                 ELSE LEAST(r.cutoff_after, r.next_effective_from - 1) END AS effective_until
FROM active_revisions r
WHERE r.frequency <> 'once'
  AND (r.cutoff_after IS NULL OR r.cutoff_after >= r.effective_from)
  AND (r.project_id IS NULL OR EXISTS (SELECT 1 FROM projects p WHERE p.id = r.project_id AND p.deleted_at IS NULL))
ORDER BY r.effective_from, r.revision;

-- name: ListScheduleRuleRevisionsForRangeByActor :many
WITH active_revisions AS (
    SELECT r.*,
           LEAD(r.effective_from) OVER (PARTITION BY r.rule_id ORDER BY r.effective_from, r.revision) AS next_effective_from
    FROM schedule_rule_revisions r
    JOIN schedule_rules b ON b.id = r.rule_id AND b.deleted_at IS NULL
    WHERE r.superseded_at IS NULL
)
SELECT r.*,
       CASE WHEN r.cutoff_after IS NULL THEN r.next_effective_from - 1
            WHEN r.next_effective_from IS NULL THEN r.cutoff_after
            ELSE LEAST(r.cutoff_after, r.next_effective_from - 1) END AS effective_until
FROM active_revisions r
JOIN schedule_rules b ON b.id = r.rule_id
WHERE r.effective_from <= (sqlc.arg(to_at)::timestamptz AT TIME ZONE r.timezone)::date
  AND (r.cutoff_after IS NULL OR r.cutoff_after >=
       (sqlc.arg(from_at)::timestamptz AT TIME ZONE r.timezone)::date - (r.local_end::date - r.local_start::date))
  AND (r.next_effective_from IS NULL OR r.next_effective_from - 1 >=
       (sqlc.arg(from_at)::timestamptz AT TIME ZONE r.timezone)::date - (r.local_end::date - r.local_start::date))
  AND (r.end_date IS NULL OR r.end_date >=
       (sqlc.arg(from_at)::timestamptz AT TIME ZONE r.timezone)::date
       - (r.local_end::date - r.local_start::date))
  AND schedule_rule_has_permission_at(b.id, r.effective_from, sqlc.arg(actor_id)::text, 'schedule', 'read')
ORDER BY b.id, r.effective_from, r.revision;
