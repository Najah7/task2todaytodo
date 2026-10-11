-- name: ListProjectMemberBlueprintRevisionCandidates :many
WITH active AS (
    SELECT revision.*, (now() AT TIME ZONE revision.timezone)::date AS local_today
    FROM schedule_blueprint_revisions revision
    JOIN schedule_blueprints blueprint ON blueprint.id = revision.blueprint_id AND blueprint.deleted_at IS NULL
    WHERE revision.superseded_at IS NULL
), current_ranked AS (
    SELECT active.*, row_number() OVER (PARTITION BY active.blueprint_id ORDER BY active.effective_from DESC, active.revision DESC) AS position
    FROM active WHERE active.effective_from <= active.local_today
), candidates AS (
    SELECT current_ranked.id AS source_revision_id, current_ranked.blueprint_id,
           GREATEST(current_ranked.effective_from, current_ranked.local_today) AS target_effective_from
    FROM current_ranked
    WHERE current_ranked.position = 1 AND current_ranked.project_id = sqlc.arg(project_id)::text
      AND current_ranked.assignee_id = sqlc.arg(member_id)::text
      AND (current_ranked.cutoff_after IS NULL OR current_ranked.cutoff_after >= current_ranked.local_today)
      AND (current_ranked.frequency <> 'once' OR current_ranked.effective_from >= current_ranked.local_today)
      AND (current_ranked.end_date IS NULL OR current_ranked.end_date >= current_ranked.local_today)
    UNION ALL
    SELECT future.id, future.blueprint_id, future.effective_from
    FROM active future
    WHERE future.effective_from > future.local_today AND future.project_id = sqlc.arg(project_id)::text
      AND future.assignee_id = sqlc.arg(member_id)::text
      AND (future.cutoff_after IS NULL OR future.cutoff_after >= future.effective_from)
      AND (future.frequency <> 'once' OR future.effective_from >= future.local_today)
      AND (future.end_date IS NULL OR future.end_date >= future.effective_from)
)
SELECT source_revision_id, blueprint_id, target_effective_from::text AS target_effective_from
FROM candidates ORDER BY blueprint_id, target_effective_from, source_revision_id;
