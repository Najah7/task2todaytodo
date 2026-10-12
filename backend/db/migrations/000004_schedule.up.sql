CREATE TABLE schedule_rules (
    id text PRIMARY KEY CHECK (id ~ '^[0-9ABCDEFGHJKMNPQRSTVWXYZ]{26}$'),
    user_id text NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT schedule_rules_id_user_id_key UNIQUE (id, user_id)
);

CREATE TABLE schedule_rule_revisions (
    id text PRIMARY KEY CHECK (id ~ '^[0-9ABCDEFGHJKMNPQRSTVWXYZ]{26}$'),
    rule_id text NOT NULL,
    user_id text NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    revision integer NOT NULL CHECK (revision > 0),
    effective_from date NOT NULL,
    recurrence_anchor_date date NOT NULL,
    local_start timestamp NOT NULL,
    local_end timestamp NOT NULL,
    all_day boolean NOT NULL DEFAULT false,
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
    CONSTRAINT schedule_rule_revisions_rule_fk FOREIGN KEY (rule_id, user_id)
        REFERENCES schedule_rules(id, user_id) ON DELETE RESTRICT,
    CONSTRAINT schedule_rule_revisions_project_user_fk FOREIGN KEY (project_id, user_id)
        REFERENCES projects(id, user_id) ON DELETE RESTRICT,
    CONSTRAINT schedule_rule_revisions_key UNIQUE (rule_id, revision),
    CONSTRAINT schedule_rule_revisions_source_key UNIQUE (id, rule_id, user_id),
    CONSTRAINT schedule_rule_revisions_local_interval CHECK (local_end > local_start),
    CONSTRAINT schedule_rule_revisions_all_day_midnight CHECK (
        NOT all_day OR (local_start::time = TIME '00:00' AND local_end::time = TIME '00:00')
    ),
    CONSTRAINT schedule_rule_revisions_end_date CHECK (end_date IS NULL OR end_date >= effective_from),
    CONSTRAINT schedule_rule_revisions_once_end CHECK (frequency <> 'once' OR end_date IS NULL),
    CONSTRAINT schedule_rule_revisions_weekdays CHECK (
        (frequency = 'weekly' AND cardinality(weekdays) > 0 AND weekdays <@ ARRAY['mon','tue','wed','thu','fri','sat','sun']::text[])
        OR (frequency <> 'weekly' AND cardinality(weekdays) = 0)
    )
);
CREATE INDEX idx_schedule_rule_revisions_active_effective
    ON schedule_rule_revisions(rule_id, effective_from)
    WHERE superseded_at IS NULL;
CREATE INDEX idx_schedule_rule_revisions_effective
    ON schedule_rule_revisions(rule_id, effective_from DESC, revision DESC)
    WHERE superseded_at IS NULL;

CREATE TABLE schedules (
    id text PRIMARY KEY CHECK (id ~ '^[0-9ABCDEFGHJKMNPQRSTVWXYZ]{26}$'),
    rule_id text NOT NULL,
    source_revision_id text NOT NULL,
    user_id text NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    occurrence_date date NOT NULL,
    start_at timestamptz NOT NULL,
    end_at timestamptz NOT NULL,
    all_day boolean NOT NULL DEFAULT false,
    title text NOT NULL CHECK (btrim(title) <> ''),
    description text,
    location text,
    project_id text,
    assignee_id text NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    timezone text NOT NULL REFERENCES timezone_master(timezone) ON DELETE RESTRICT,
    completed boolean NOT NULL DEFAULT false,
    skipped_at timestamptz,
    deleted_at timestamptz,
    revision integer NOT NULL DEFAULT 1 CHECK (revision > 0),
    changed_by text NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT schedules_rule_user_fk FOREIGN KEY (rule_id, user_id)
        REFERENCES schedule_rules(id, user_id) ON DELETE RESTRICT,
    CONSTRAINT schedules_source_rule_revision_fk FOREIGN KEY (source_revision_id, rule_id, user_id)
        REFERENCES schedule_rule_revisions(id, rule_id, user_id) ON DELETE RESTRICT,
    CONSTRAINT schedules_project_user_fk FOREIGN KEY (project_id, user_id)
        REFERENCES projects(id, user_id) ON DELETE RESTRICT,
    CONSTRAINT schedules_rule_occurrence_key UNIQUE (rule_id, occurrence_date),
    CONSTRAINT schedules_end_after_start CHECK (end_at > start_at),
    CONSTRAINT schedules_all_day_midnight CHECK (
        NOT all_day OR (
            start_at AT TIME ZONE timezone = date_trunc('day', start_at AT TIME ZONE timezone)
            AND end_at AT TIME ZONE timezone = date_trunc('day', end_at AT TIME ZONE timezone)
        )
    ),
    CONSTRAINT schedules_id_rule_key UNIQUE (id, rule_id)
);
CREATE INDEX idx_schedules_user_assignee_start_id_live
    ON schedules(user_id, assignee_id, start_at, id) WHERE deleted_at IS NULL;
CREATE INDEX idx_schedules_project_start_id_live
    ON schedules(project_id, start_at, id) WHERE deleted_at IS NULL;
CREATE INDEX idx_schedules_rule_occurrence ON schedules(rule_id, occurrence_date, id);

CREATE TABLE schedule_revisions (
    schedule_id text NOT NULL REFERENCES schedules(id) ON DELETE RESTRICT,
    revision integer NOT NULL CHECK (revision > 0),
    source_revision_id text NOT NULL REFERENCES schedule_rule_revisions(id) ON DELETE RESTRICT,
    user_id text NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    occurrence_date date NOT NULL,
    start_at timestamptz NOT NULL,
    end_at timestamptz NOT NULL,
    all_day boolean NOT NULL DEFAULT false,
    title text NOT NULL,
    description text,
    location text,
    project_id text,
    assignee_id text NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    timezone text NOT NULL REFERENCES timezone_master(timezone) ON DELETE RESTRICT,
    completed boolean NOT NULL,
    skipped_at timestamptz,
    deleted_at timestamptz,
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
        timezone, all_day, completed, skipped_at, deleted_at, changed_by, changed_at
    ) VALUES (
        NEW.id, NEW.revision, NEW.source_revision_id, NEW.user_id, NEW.occurrence_date,
        NEW.start_at, NEW.end_at, NEW.title, NEW.description, NEW.location, NEW.project_id,
        NEW.assignee_id, NEW.timezone, NEW.all_day, NEW.completed, NEW.skipped_at, NEW.deleted_at,
        NEW.changed_by, NEW.updated_at
    );
    RETURN NEW;
END;
$$;
CREATE TRIGGER schedules_revision_before_write
    BEFORE INSERT OR UPDATE ON schedules FOR EACH ROW EXECUTE FUNCTION prepare_schedule_revision();
CREATE TRIGGER schedules_revision_snapshot
    AFTER INSERT OR UPDATE ON schedules FOR EACH ROW EXECUTE FUNCTION snapshot_schedule_revision();

CREATE TABLE schedule_exceptions (
    rule_id text NOT NULL,
    occurrence_date date NOT NULL,
    source_revision_id text NOT NULL,
    user_id text NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    start_at timestamptz NOT NULL,
    end_at timestamptz NOT NULL,
    all_day boolean NOT NULL,
    title text NOT NULL CHECK (btrim(title) <> ''),
    description text,
    location text,
    project_id text,
    assignee_id text NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    timezone text NOT NULL REFERENCES timezone_master(timezone) ON DELETE RESTRICT,
    completed boolean NOT NULL DEFAULT false,
    skipped_at timestamptz,
    deleted_at timestamptz,
    changed_by text NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (rule_id, occurrence_date),
    CONSTRAINT schedule_exceptions_rule_user_fk FOREIGN KEY (rule_id, user_id)
        REFERENCES schedule_rules(id, user_id) ON DELETE RESTRICT,
    CONSTRAINT schedule_exceptions_source_revision_fk FOREIGN KEY (source_revision_id, rule_id, user_id)
        REFERENCES schedule_rule_revisions(id, rule_id, user_id) ON DELETE RESTRICT,
    CONSTRAINT schedule_exceptions_schedule_fk FOREIGN KEY (rule_id, occurrence_date)
        REFERENCES schedules(rule_id, occurrence_date) ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED,
    CONSTRAINT schedule_exceptions_project_user_fk FOREIGN KEY (project_id, user_id)
        REFERENCES projects(id, user_id) ON DELETE RESTRICT,
    CONSTRAINT schedule_exceptions_end_after_start CHECK (end_at > start_at),
    CONSTRAINT schedule_exceptions_all_day_midnight CHECK (
        NOT all_day OR (
            start_at AT TIME ZONE timezone = date_trunc('day', start_at AT TIME ZONE timezone)
            AND end_at AT TIME ZONE timezone = date_trunc('day', end_at AT TIME ZONE timezone)
        )
    )
);
CREATE INDEX idx_schedule_exceptions_range ON schedule_exceptions(start_at, end_at);

CREATE TABLE schedule_project_pause_intervals (
    project_id text NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    rule_id text NOT NULL REFERENCES schedule_rules(id) ON DELETE RESTRICT,
    paused_at timestamptz NOT NULL,
    resumed_at timestamptz,
    paused_by text NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    resumed_by text REFERENCES users(id) ON DELETE RESTRICT,
    PRIMARY KEY (project_id, rule_id, paused_at),
    CHECK (resumed_at IS NULL OR resumed_at >= paused_at)
);
CREATE UNIQUE INDEX idx_schedule_project_pause_active
    ON schedule_project_pause_intervals(project_id, rule_id) WHERE resumed_at IS NULL;
CREATE INDEX idx_schedule_project_pause_interval
    ON schedule_project_pause_intervals(rule_id, project_id, paused_at, resumed_at);

CREATE TABLE schedule_backfill_checkpoints (
    revision_id text PRIMARY KEY REFERENCES schedule_rule_revisions(id) ON DELETE RESTRICT,
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

CREATE FUNCTION schedule_rule_has_permission(rule_key text, actor_key text, resource_key text, action_key action)
RETURNS boolean LANGUAGE sql STABLE AS $$
    SELECT EXISTS (
        SELECT 1
        FROM schedule_rules rule
        WHERE rule.id = rule_key AND rule.deleted_at IS NULL
          AND EXISTS (
              SELECT 1
              FROM LATERAL (
                  SELECT revision.* FROM schedule_rule_revisions revision
                  WHERE revision.rule_id = rule.id
                  ORDER BY
                    CASE WHEN revision.superseded_at IS NULL THEN 0 ELSE 1 END,
                    CASE WHEN revision.effective_from <= (now() AT TIME ZONE revision.timezone)::date THEN 0 ELSE 1 END,
                    CASE WHEN revision.effective_from <= (now() AT TIME ZONE revision.timezone)::date THEN revision.effective_from END DESC,
                    CASE WHEN revision.effective_from > (now() AT TIME ZONE revision.timezone)::date THEN revision.effective_from END,
                    revision.revision DESC LIMIT 1
              ) current_revision
              LEFT JOIN projects parent ON parent.id = current_revision.project_id
              WHERE (current_revision.project_id IS NULL AND rule.user_id = actor_key)
                 OR (current_revision.project_id IS NOT NULL AND parent.deleted_at IS NULL AND
                     (rule.user_id = actor_key OR project_has_permission(current_revision.project_id, actor_key, resource_key, action_key)))
          )
    )
$$;

CREATE FUNCTION schedule_rule_has_permission_at(rule_key text, occurrence_key date, actor_key text, resource_key text, action_key action)
RETURNS boolean LANGUAGE sql STABLE AS $$
    SELECT EXISTS (
        SELECT 1 FROM schedule_rules rule
        JOIN LATERAL (
            SELECT revision.* FROM schedule_rule_revisions revision
            WHERE revision.rule_id = rule.id AND revision.superseded_at IS NULL
              AND revision.effective_from <= occurrence_key
            ORDER BY revision.effective_from DESC, revision.revision DESC LIMIT 1
        ) effective ON true
        LEFT JOIN projects parent ON parent.id = effective.project_id
        WHERE rule.id = rule_key AND rule.deleted_at IS NULL
          AND ((effective.project_id IS NULL AND rule.user_id = actor_key)
            OR (effective.project_id IS NOT NULL AND parent.deleted_at IS NULL AND
                (rule.user_id = actor_key OR project_has_permission(effective.project_id, actor_key, resource_key, action_key))))
          AND (effective.cutoff_after IS NULL OR effective.cutoff_after >= occurrence_key)
    )
$$;
