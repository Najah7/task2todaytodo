-- name: PauseScheduleRulesForProject :exec
WITH current_or_planned_revisions AS (
    SELECT revision.*
    FROM schedule_rule_revisions revision
    JOIN schedule_rules rule ON rule.id = revision.rule_id
    WHERE revision.superseded_at IS NULL
      AND (
          revision.effective_from > (sqlc.arg(paused_at)::timestamptz AT TIME ZONE revision.timezone)::date
          OR ((revision.cutoff_after IS NULL OR revision.cutoff_after >= (sqlc.arg(paused_at)::timestamptz AT TIME ZONE revision.timezone)::date)
              AND NOT EXISTS (
              SELECT 1 FROM schedule_rule_revisions later
              WHERE later.rule_id = revision.rule_id
                AND later.superseded_at IS NULL
                AND later.effective_from <= (sqlc.arg(paused_at)::timestamptz AT TIME ZONE revision.timezone)::date
                AND later.effective_from > revision.effective_from
              ))
      )
      AND rule.deleted_at IS NULL
), inserted AS (
    INSERT INTO schedule_project_pause_intervals (
        project_id, rule_id, paused_at, paused_by
    )
    SELECT sqlc.arg(project_id)::text, current.rule_id,
           sqlc.arg(paused_at)::timestamptz, sqlc.arg(actor_id)::text
    FROM current_or_planned_revisions current
    WHERE current.project_id = sqlc.arg(project_id)::text
      AND current.frequency <> 'once'
      AND (current.end_date IS NULL OR current.end_date >= (sqlc.arg(paused_at)::timestamptz AT TIME ZONE current.timezone)::date)
    ON CONFLICT (project_id, rule_id) WHERE resumed_at IS NULL DO NOTHING
)
SELECT 1;

-- name: ResumeScheduleRulesForProject :exec
UPDATE schedule_project_pause_intervals
SET resumed_at = sqlc.arg(resumed_at)::timestamptz,
    resumed_by = sqlc.arg(actor_id)::text
WHERE project_id = sqlc.arg(project_id)::text
  AND resumed_at IS NULL;
