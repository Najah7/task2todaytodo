CREATE TABLE schedule_blueprints (
    id text PRIMARY KEY CHECK (id ~ '^[0-9ABCDEFGHJKMNPQRSTVWXYZ]{26}$'),
    user_id text NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT schedule_blueprints_id_user_id_key UNIQUE (id, user_id)
);

CREATE TABLE schedule_blueprint_revisions (
    id text PRIMARY KEY CHECK (id ~ '^[0-9ABCDEFGHJKMNPQRSTVWXYZ]{26}$'),
    blueprint_id text NOT NULL,
    user_id text NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    revision integer NOT NULL CHECK (revision > 0),
    effective_from date NOT NULL,
    recurrence_anchor_date date NOT NULL,
    local_start timestamp NOT NULL,
    local_end timestamp NOT NULL,
    timezone text NOT NULL REFERENCES timezone_master(timezone) ON DELETE RESTRICT,
    frequency text NOT NULL CHECK (frequency IN ('once', 'daily', 'weekly', 'monthly', 'yearly')),
    interval_count integer NOT NULL CHECK ((frequency = 'once' AND interval_count = 0) OR (frequency <> 'once' AND interval_count > 0)),
    weekdays text[] NOT NULL DEFAULT '{}',
    end_date date,
    title text NOT NULL CHECK (btrim(title) <> ''),
    description text,
    location text,
    project_id text,
    assignee_id text NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    superseded_at timestamptz,
    cutoff_after date,
    changed_by text NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT schedule_blueprint_revisions_blueprint_fk FOREIGN KEY (blueprint_id, user_id)
        REFERENCES schedule_blueprints(id, user_id) ON DELETE RESTRICT,
    CONSTRAINT schedule_blueprint_revisions_project_user_fk FOREIGN KEY (project_id, user_id)
        REFERENCES projects(id, user_id) ON DELETE RESTRICT,
    CONSTRAINT schedule_blueprint_revisions_key UNIQUE (blueprint_id, revision),
    CONSTRAINT schedule_blueprint_revisions_source_key UNIQUE (id, blueprint_id, user_id),
    CONSTRAINT schedule_blueprint_revisions_local_interval CHECK (local_end > local_start),
    CONSTRAINT schedule_blueprint_revisions_end_date CHECK (end_date IS NULL OR end_date >= effective_from),
    CONSTRAINT schedule_blueprint_revisions_once_end CHECK (frequency <> 'once' OR end_date IS NULL),
    CONSTRAINT schedule_blueprint_revisions_weekdays CHECK (
        (frequency = 'weekly' AND cardinality(weekdays) > 0 AND weekdays <@ ARRAY['mon','tue','wed','thu','fri','sat','sun']::text[])
        OR (frequency <> 'weekly' AND cardinality(weekdays) = 0)
    )
);
CREATE UNIQUE INDEX idx_schedule_blueprint_revisions_active_effective
    ON schedule_blueprint_revisions(blueprint_id, effective_from)
    WHERE superseded_at IS NULL;
CREATE INDEX idx_schedule_blueprint_revisions_effective
    ON schedule_blueprint_revisions(blueprint_id, effective_from DESC, revision DESC)
    WHERE superseded_at IS NULL;

CREATE TABLE schedules (
    id text PRIMARY KEY CHECK (id ~ '^[0-9ABCDEFGHJKMNPQRSTVWXYZ]{26}$'),
    blueprint_id text NOT NULL,
    source_revision_id text NOT NULL,
    user_id text NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    occurrence_date date NOT NULL,
    start_at timestamptz NOT NULL,
    end_at timestamptz NOT NULL,
    title text NOT NULL CHECK (btrim(title) <> ''),
    description text,
    location text,
    project_id text,
    assignee_id text NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    timezone text NOT NULL REFERENCES timezone_master(timezone) ON DELETE RESTRICT,
    completed boolean NOT NULL DEFAULT false,
    skipped_at timestamptz,
    deleted_at timestamptz,
    is_override boolean NOT NULL DEFAULT false,
    revision integer NOT NULL DEFAULT 1 CHECK (revision > 0),
    changed_by text NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT schedules_blueprint_user_fk FOREIGN KEY (blueprint_id, user_id)
        REFERENCES schedule_blueprints(id, user_id) ON DELETE RESTRICT,
    CONSTRAINT schedules_source_revision_fk FOREIGN KEY (source_revision_id, blueprint_id, user_id)
        REFERENCES schedule_blueprint_revisions(id, blueprint_id, user_id) ON DELETE RESTRICT,
    CONSTRAINT schedules_project_user_fk FOREIGN KEY (project_id, user_id)
        REFERENCES projects(id, user_id) ON DELETE RESTRICT,
    CONSTRAINT schedules_blueprint_occurrence_key UNIQUE (blueprint_id, occurrence_date),
    CONSTRAINT schedules_end_after_start CHECK (end_at > start_at),
    CONSTRAINT schedules_id_blueprint_key UNIQUE (id, blueprint_id)
);
CREATE INDEX idx_schedules_user_assignee_start_id_live
    ON schedules(user_id, assignee_id, start_at, id) WHERE deleted_at IS NULL;
CREATE INDEX idx_schedules_project_start_id_live
    ON schedules(project_id, start_at, id) WHERE deleted_at IS NULL;
CREATE INDEX idx_schedules_blueprint_occurrence
    ON schedules(blueprint_id, occurrence_date, id);

CREATE TABLE schedule_revisions (
    schedule_id text NOT NULL REFERENCES schedules(id) ON DELETE RESTRICT,
    revision integer NOT NULL CHECK (revision > 0),
    source_revision_id text NOT NULL REFERENCES schedule_blueprint_revisions(id) ON DELETE RESTRICT,
    user_id text NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    occurrence_date date NOT NULL,
    start_at timestamptz NOT NULL,
    end_at timestamptz NOT NULL,
    title text NOT NULL,
    description text,
    location text,
    project_id text,
    assignee_id text NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    timezone text NOT NULL REFERENCES timezone_master(timezone) ON DELETE RESTRICT,
    completed boolean NOT NULL,
    skipped_at timestamptz,
    deleted_at timestamptz,
    is_override boolean NOT NULL,
    changed_by text NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    changed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (schedule_id, revision)
);

CREATE FUNCTION prepare_schedule_revision() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        NEW.revision := 1;
    ELSE
        NEW.revision := OLD.revision + 1;
        NEW.updated_at := now();
    END IF;
    RETURN NEW;
END;
$$;

CREATE FUNCTION snapshot_schedule_revision() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO schedule_revisions (
        schedule_id, revision, source_revision_id, user_id, occurrence_date,
        start_at, end_at, title, description, location, project_id, assignee_id,
        timezone, completed, skipped_at, deleted_at, is_override, changed_by, changed_at
    ) VALUES (
        NEW.id, NEW.revision, NEW.source_revision_id, NEW.user_id, NEW.occurrence_date,
        NEW.start_at, NEW.end_at, NEW.title, NEW.description, NEW.location, NEW.project_id,
        NEW.assignee_id, NEW.timezone, NEW.completed, NEW.skipped_at, NEW.deleted_at,
        NEW.is_override, NEW.changed_by, NEW.updated_at
    );
    RETURN NEW;
END;
$$;
CREATE TRIGGER schedules_revision_before_write
    BEFORE INSERT OR UPDATE ON schedules FOR EACH ROW EXECUTE FUNCTION prepare_schedule_revision();
CREATE TRIGGER schedules_revision_snapshot
    AFTER INSERT OR UPDATE ON schedules FOR EACH ROW EXECUTE FUNCTION snapshot_schedule_revision();

CREATE TABLE schedule_project_pause_intervals (
    project_id text NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    blueprint_id text NOT NULL REFERENCES schedule_blueprints(id) ON DELETE RESTRICT,
    paused_at timestamptz NOT NULL,
    resumed_at timestamptz,
    paused_by text NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    resumed_by text REFERENCES users(id) ON DELETE RESTRICT,
    PRIMARY KEY (project_id, blueprint_id, paused_at),
    CHECK (resumed_at IS NULL OR resumed_at >= paused_at)
);
CREATE UNIQUE INDEX idx_schedule_project_pause_active
    ON schedule_project_pause_intervals(project_id, blueprint_id) WHERE resumed_at IS NULL;
CREATE INDEX idx_schedule_project_pause_interval
    ON schedule_project_pause_intervals(blueprint_id, project_id, paused_at, resumed_at);

CREATE TABLE schedule_backfill_checkpoints (
    revision_id text PRIMARY KEY REFERENCES schedule_blueprint_revisions(id) ON DELETE RESTRICT,
    through_date date NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE FUNCTION schedule_has_permission(schedule_key text, actor_key text, resource_key text, action_key action)
RETURNS boolean LANGUAGE sql STABLE AS $$
    SELECT EXISTS (
        SELECT 1 FROM schedules AS s
        WHERE s.id = schedule_key AND s.deleted_at IS NULL
          AND (s.project_id IS NULL OR EXISTS (
              SELECT 1 FROM projects AS parent WHERE parent.id = s.project_id AND parent.deleted_at IS NULL
          ))
          AND (s.user_id = actor_key OR (s.project_id IS NOT NULL
              AND project_has_permission(s.project_id, actor_key, resource_key, action_key)))
    )
$$;

CREATE FUNCTION schedule_occurrence_has_permission(schedule_key text, actor_key text, resource_key text, action_key action)
RETURNS boolean LANGUAGE sql STABLE AS $$
    SELECT EXISTS (
        SELECT 1 FROM schedules AS s
        WHERE s.id = schedule_key
          AND (s.project_id IS NULL OR EXISTS (
              SELECT 1 FROM projects AS parent WHERE parent.id = s.project_id AND parent.deleted_at IS NULL
          ))
          AND (s.user_id = actor_key OR (s.project_id IS NOT NULL
              AND project_has_permission(s.project_id, actor_key, resource_key, action_key)))
    )
$$;

CREATE FUNCTION schedule_blueprint_has_permission(blueprint_key text, actor_key text, resource_key text, action_key action)
RETURNS boolean LANGUAGE sql STABLE AS $$
    SELECT EXISTS (
        SELECT 1
        FROM schedule_blueprints blueprint
        WHERE blueprint.id = blueprint_key AND blueprint.deleted_at IS NULL
          AND EXISTS (
              SELECT 1
              FROM LATERAL (
                  SELECT revision.* FROM schedule_blueprint_revisions revision
                  WHERE revision.blueprint_id = blueprint.id
                  ORDER BY
                    CASE WHEN revision.superseded_at IS NULL THEN 0 ELSE 1 END,
                    CASE WHEN revision.effective_from <= (now() AT TIME ZONE revision.timezone)::date THEN 0 ELSE 1 END,
                    CASE WHEN revision.effective_from <= (now() AT TIME ZONE revision.timezone)::date THEN revision.effective_from END DESC,
                    CASE WHEN revision.effective_from > (now() AT TIME ZONE revision.timezone)::date THEN revision.effective_from END,
                    revision.revision DESC LIMIT 1
              ) current_revision
              LEFT JOIN projects parent ON parent.id = current_revision.project_id
              WHERE (current_revision.project_id IS NULL AND blueprint.user_id = actor_key)
                 OR (current_revision.project_id IS NOT NULL AND parent.deleted_at IS NULL AND
                     (blueprint.user_id = actor_key OR project_has_permission(current_revision.project_id, actor_key, resource_key, action_key)))
          )
    )
$$;

CREATE FUNCTION schedule_blueprint_has_permission_at(blueprint_key text, occurrence_key date, actor_key text, resource_key text, action_key action)
RETURNS boolean LANGUAGE sql STABLE AS $$
    SELECT EXISTS (
        SELECT 1 FROM schedule_blueprints blueprint
        JOIN LATERAL (
            SELECT revision.* FROM schedule_blueprint_revisions revision
            WHERE revision.blueprint_id = blueprint.id AND revision.superseded_at IS NULL
              AND revision.effective_from <= occurrence_key
            ORDER BY revision.effective_from DESC, revision.revision DESC LIMIT 1
        ) effective ON true
        LEFT JOIN projects parent ON parent.id = effective.project_id
        WHERE blueprint.id = blueprint_key AND blueprint.deleted_at IS NULL
          AND ((effective.project_id IS NULL AND blueprint.user_id = actor_key)
            OR (effective.project_id IS NOT NULL AND parent.deleted_at IS NULL AND
                (blueprint.user_id = actor_key OR project_has_permission(effective.project_id, actor_key, resource_key, action_key))))
          AND (effective.cutoff_after IS NULL OR effective.cutoff_after >= occurrence_key)
    )
$$;
