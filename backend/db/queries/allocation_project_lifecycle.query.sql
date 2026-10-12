-- name: ListProjectAllocationUsersForLifecycle :many
SELECT DISTINCT owners.user_id
FROM (
    SELECT rule.user_id
    FROM project_time_allocation_rules rule
    JOIN LATERAL (
        SELECT revision.project_id
        FROM project_time_allocation_rule_revisions revision
        WHERE revision.rule_id = rule.id AND revision.superseded_at IS NULL
        ORDER BY revision.effective_from DESC, revision.revision DESC
        LIMIT 1
    ) current_revision ON true
    WHERE current_revision.project_id = sqlc.arg(project_id)::text
      AND rule.deleted_at IS NULL
    UNION
    SELECT rule.user_id
    FROM project_time_allocation_pause_intervals pause
    JOIN project_time_allocation_rules rule ON rule.id = pause.rule_id
    WHERE pause.project_id = sqlc.arg(project_id)::text
      AND pause.resumed_at IS NULL
      AND rule.deleted_at IS NULL
) owners
ORDER BY owners.user_id;
