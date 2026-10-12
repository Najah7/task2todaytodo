-- name: ReassignProjectMemberRuleRevisions :execrows
WITH active AS (
    SELECT revision.*, (now() AT TIME ZONE revision.timezone)::date AS local_today
    FROM schedule_rule_revisions revision
    JOIN schedule_rules rule ON rule.id = revision.rule_id AND rule.deleted_at IS NULL
    WHERE revision.superseded_at IS NULL
), current_ranked AS (
    SELECT active.*, row_number() OVER (PARTITION BY active.rule_id ORDER BY active.effective_from DESC, active.revision DESC) AS position
    FROM active WHERE active.effective_from <= active.local_today
), candidate_keys AS (
    SELECT current_ranked.id AS source_revision_id,
           GREATEST(current_ranked.effective_from, current_ranked.local_today) AS target_effective_from
    FROM current_ranked
    WHERE current_ranked.position = 1 AND current_ranked.project_id = sqlc.arg(project_id)::text
      AND current_ranked.assignee_id = sqlc.arg(member_id)::text
      AND (current_ranked.cutoff_after IS NULL OR current_ranked.cutoff_after >= current_ranked.local_today)
      AND (current_ranked.frequency <> 'once' OR current_ranked.effective_from >= current_ranked.local_today)
      AND (current_ranked.end_date IS NULL OR current_ranked.end_date >= current_ranked.local_today)
    UNION ALL
    SELECT future.id, future.effective_from
    FROM active future
    WHERE future.effective_from > future.local_today AND future.project_id = sqlc.arg(project_id)::text
      AND future.assignee_id = sqlc.arg(member_id)::text
      AND (future.cutoff_after IS NULL OR future.cutoff_after >= future.effective_from)
      AND (future.frequency <> 'once' OR future.effective_from >= future.local_today)
      AND (future.end_date IS NULL OR future.end_date >= future.effective_from)
), candidates AS (
    SELECT active.*, candidate_keys.target_effective_from
    FROM candidate_keys
    JOIN active ON active.id = candidate_keys.source_revision_id
), numbered AS (
    SELECT candidates.*,
           row_number() OVER (ORDER BY rule_id, target_effective_from, id) AS ordinal,
           row_number() OVER (PARTITION BY rule_id ORDER BY target_effective_from, id) AS rule_ordinal,
           (SELECT max(all_revisions.revision) FROM schedule_rule_revisions all_revisions
            WHERE all_revisions.rule_id = candidates.rule_id) AS max_revision
    FROM candidates
), provided_ids AS (
    SELECT value AS revision_id, ordinality AS ordinal
    FROM unnest(sqlc.arg(revision_ids)::text[]) WITH ORDINALITY AS ids(value, ordinality)
), superseded AS (
    UPDATE schedule_rule_revisions old SET superseded_at = now()
    FROM numbered
    WHERE old.id = numbered.id AND old.effective_from >= numbered.local_today
    RETURNING old.id
), inserted AS (
    INSERT INTO schedule_rule_revisions (
        id, rule_id, user_id, revision, effective_from, recurrence_anchor_date,
        local_start, local_end, all_day, timezone, frequency, interval_count, weekdays, end_date,
        cutoff_after, title, description, location, project_id, assignee_id, changed_by
    )
    SELECT provided_ids.revision_id, numbered.rule_id, numbered.user_id,
           numbered.max_revision + numbered.rule_ordinal, numbered.target_effective_from,
           numbered.recurrence_anchor_date, numbered.local_start, numbered.local_end,
           numbered.all_day, numbered.timezone, numbered.frequency, numbered.interval_count, numbered.weekdays,
           numbered.end_date, numbered.cutoff_after, numbered.title, numbered.description,
           numbered.location, numbered.project_id, owner.user_id, sqlc.arg(actor_id)::text
    FROM numbered JOIN provided_ids USING (ordinal)
    JOIN projects owner ON owner.id = numbered.project_id
    WHERE (SELECT count(*) FROM superseded) >= 0
      AND cardinality(sqlc.arg(revision_ids)::text[]) = (SELECT count(*) FROM numbered)
    RETURNING id
)
SELECT inserted.id FROM inserted;
