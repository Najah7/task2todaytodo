-- name: CreateScheduleBlueprintByActor :one
WITH scope AS (
    SELECT p.id AS project_id, p.user_id AS owner_id,
           COALESCE(NULLIF(sqlc.arg(assignee_id)::text, ''), p.user_id) AS assignee_id
    FROM projects p
    WHERE p.id = sqlc.narg(project_id)::text
      AND p.deleted_at IS NULL
      AND project_has_permission(p.id, sqlc.arg(actor_id)::text, 'schedule', 'create')
      AND (COALESCE(NULLIF(sqlc.arg(assignee_id)::text, ''), p.user_id) = p.user_id OR EXISTS (
          SELECT 1 FROM project_members pm WHERE pm.project_id = p.id AND pm.user_id = sqlc.arg(assignee_id)::text
      ))
    UNION ALL
    SELECT NULL::text, sqlc.arg(actor_id)::text,
           COALESCE(NULLIF(sqlc.arg(assignee_id)::text, ''), sqlc.arg(actor_id)::text)
    WHERE sqlc.narg(project_id)::text IS NULL
      AND COALESCE(NULLIF(sqlc.arg(assignee_id)::text, ''), sqlc.arg(actor_id)::text) = sqlc.arg(actor_id)::text
)
INSERT INTO schedule_blueprints(id, user_id)
SELECT sqlc.arg(blueprint_id)::text, scope.owner_id FROM scope
RETURNING id, user_id, created_at;

-- name: CreateScheduleBlueprintRevisionByActor :one
INSERT INTO schedule_blueprint_revisions (
    id, blueprint_id, user_id, revision, effective_from, recurrence_anchor_date,
    local_start, local_end, all_day, timezone, frequency, interval_count, weekdays, end_date,
    title, description, location, project_id, assignee_id, changed_by
)
SELECT sqlc.arg(revision_id)::text, b.id, b.user_id, 1, sqlc.arg(effective_from)::date,
       sqlc.arg(recurrence_anchor_date)::date, sqlc.arg(local_start)::timestamp,
       sqlc.arg(local_end)::timestamp, sqlc.arg(all_day)::boolean, sqlc.arg(timezone)::text, sqlc.arg(frequency)::text,
       sqlc.arg(interval_count)::integer, sqlc.arg(weekdays)::text[], sqlc.narg(end_date)::date,
       sqlc.arg(title)::text, sqlc.narg(description)::text, sqlc.narg(location)::text,
       sqlc.narg(project_id)::text, COALESCE(NULLIF(sqlc.arg(assignee_id)::text, ''), b.user_id), sqlc.arg(actor_id)::text
FROM schedule_blueprints b
WHERE b.id = sqlc.arg(blueprint_id)::text
  AND (b.user_id = sqlc.arg(actor_id)::text OR (
      sqlc.narg(project_id)::text IS NOT NULL
      AND project_has_permission(sqlc.narg(project_id)::text, sqlc.arg(actor_id)::text, 'schedule', 'create')
  ))
RETURNING *;

-- name: DeleteScheduleBlueprintFollowingByActor :execrows
WITH locked AS (
    SELECT b.id
    FROM schedule_blueprints b
    WHERE b.id = sqlc.arg(blueprint_id)::text
      AND schedule_blueprint_has_permission_at(b.id, sqlc.arg(occurrence_date)::date, sqlc.arg(actor_id)::text, 'schedule', 'delete')
      AND (SELECT COALESCE(MAX(revision), 0) FROM schedule_blueprint_revisions WHERE blueprint_id = b.id) = sqlc.arg(expected_revision)::integer
    FOR UPDATE
), superseded AS (
    UPDATE schedule_blueprint_revisions r
    SET superseded_at = now()
    FROM locked
    WHERE r.blueprint_id = locked.id AND r.superseded_at IS NULL
      AND r.effective_from >= sqlc.arg(occurrence_date)::date
    RETURNING r.id
), truncated AS (
    UPDATE schedule_blueprint_revisions preceding
    SET cutoff_after = LEAST(COALESCE(preceding.cutoff_after, sqlc.arg(occurrence_date)::date - 1), sqlc.arg(occurrence_date)::date - 1)
    FROM locked
    WHERE preceding.id = (
        SELECT prior.id FROM schedule_blueprint_revisions prior
        WHERE prior.blueprint_id = locked.id AND prior.superseded_at IS NULL
          AND prior.effective_from < sqlc.arg(occurrence_date)::date
        ORDER BY prior.effective_from DESC, prior.revision DESC
        LIMIT 1
    )
    RETURNING preceding.id
), removed AS (
    UPDATE schedules s
    SET deleted_at = now(), changed_by = sqlc.arg(actor_id)::text, updated_at = now()
    FROM locked
    WHERE s.blueprint_id = locked.id AND s.occurrence_date >= sqlc.arg(occurrence_date)::date
      AND s.deleted_at IS NULL
    RETURNING s.id
), cancellation AS (
    INSERT INTO schedule_blueprint_revisions (
        id, blueprint_id, user_id, revision, effective_from, recurrence_anchor_date,
        local_start, local_end, all_day, timezone, frequency, interval_count, weekdays, end_date,
        title, description, location, project_id, assignee_id, superseded_at, cutoff_after, changed_by
    )
    SELECT sqlc.arg(revision_id)::text, source.blueprint_id, source.user_id,
           (SELECT COALESCE(MAX(all_revisions.revision), 0) + 1 FROM schedule_blueprint_revisions all_revisions
            WHERE all_revisions.blueprint_id = source.blueprint_id),
           sqlc.arg(occurrence_date)::date, source.recurrence_anchor_date,
           source.local_start, source.local_end, source.all_day, source.timezone, source.frequency,
           source.interval_count, source.weekdays, source.end_date, source.title,
           source.description, source.location, source.project_id, source.assignee_id,
           now(), sqlc.arg(occurrence_date)::date - 1, sqlc.arg(actor_id)::text
    FROM schedule_blueprint_revisions source
    JOIN locked ON locked.id = source.blueprint_id
    WHERE source.superseded_at IS NULL
      AND source.effective_from <= sqlc.arg(occurrence_date)::date
      AND (source.cutoff_after IS NULL OR source.cutoff_after >= sqlc.arg(occurrence_date)::date)
      AND NOT EXISTS (
          SELECT 1 FROM schedule_blueprint_revisions later
          WHERE later.blueprint_id = source.blueprint_id AND later.superseded_at IS NULL
            AND later.effective_from <= sqlc.arg(occurrence_date)::date
            AND (later.effective_from, later.revision) > (source.effective_from, source.revision)
      )
    RETURNING id
)
SELECT locked.id FROM locked
WHERE (SELECT count(*) FROM superseded) >= 0
  AND (SELECT count(*) FROM truncated) >= 0
  AND (SELECT count(*) FROM removed) >= 0
  AND (SELECT count(*) FROM cancellation) >= 0;

-- name: PublishScheduleBlueprintRevisionByActor :one
WITH locked AS (
    SELECT b.id, b.user_id
    FROM schedule_blueprints b
    WHERE b.id = sqlc.arg(blueprint_id)::text
      AND schedule_blueprint_has_permission_at(b.id, sqlc.arg(effective_from)::date,
          sqlc.arg(actor_id)::text, sqlc.arg(resource_id)::text, sqlc.arg(action)::action)
      AND (SELECT COALESCE(MAX(revision), 0) FROM schedule_blueprint_revisions WHERE blueprint_id = b.id) = sqlc.arg(expected_revision)::integer
    FOR UPDATE
), superseded AS (
    UPDATE schedule_blueprint_revisions old
    SET superseded_at = now()
    FROM locked
    WHERE old.blueprint_id = locked.id AND old.superseded_at IS NULL
      AND old.effective_from >= sqlc.arg(effective_from)::date
    RETURNING old.id
), next_revision AS (
    SELECT COALESCE(MAX(r.revision), 0) + 1 AS revision, (SELECT count(*) FROM superseded) AS superseded_count
    FROM schedule_blueprint_revisions r, locked
    WHERE r.blueprint_id = locked.id
)
INSERT INTO schedule_blueprint_revisions (
    id, blueprint_id, user_id, revision, effective_from, recurrence_anchor_date,
    local_start, local_end, all_day, timezone, frequency, interval_count, weekdays, end_date, cutoff_after,
    title, description, location, project_id, assignee_id, changed_by
)
SELECT sqlc.arg(revision_id)::text, locked.id, locked.user_id, next_revision.revision,
       sqlc.arg(effective_from)::date, sqlc.arg(recurrence_anchor_date)::date,
       sqlc.arg(local_start)::timestamp, sqlc.arg(local_end)::timestamp, sqlc.arg(all_day)::boolean, sqlc.arg(timezone)::text,
       sqlc.arg(frequency)::text, sqlc.arg(interval_count)::integer, sqlc.arg(weekdays)::text[],
       sqlc.narg(end_date)::date, sqlc.narg(cutoff_after)::date, sqlc.arg(title)::text, sqlc.narg(description)::text,
       sqlc.narg(location)::text, sqlc.narg(project_id)::text, sqlc.arg(assignee_id)::text,
       sqlc.arg(actor_id)::text
FROM locked, next_revision
WHERE next_revision.superseded_count >= 0
RETURNING *;
