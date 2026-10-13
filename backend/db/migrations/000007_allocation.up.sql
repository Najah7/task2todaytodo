CREATE TABLE project_time_allocation_rules (
    id text PRIMARY KEY CHECK (id ~ '^[0-9ABCDEFGHJKMNPQRSTVWXYZ]{26}$'),
    user_id text NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    project_id text NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    stopped_after date,
    pause_reason text,
    paused_at timestamptz,
    CONSTRAINT project_time_allocation_rules_id_user_key UNIQUE (id, user_id)
);
CREATE INDEX idx_project_time_allocation_rules_user_project
    ON project_time_allocation_rules(user_id, project_id) WHERE deleted_at IS NULL;

CREATE TABLE project_time_allocation_plans (
    user_id text NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    project_id text NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    target_minutes integer NOT NULL CHECK (target_minutes >= 0),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, project_id)
);

CREATE TABLE project_time_allocation_rule_revisions (
    id text PRIMARY KEY CHECK (id ~ '^[0-9ABCDEFGHJKMNPQRSTVWXYZ]{26}$'),
    rule_id text NOT NULL REFERENCES project_time_allocation_rules(id) ON DELETE RESTRICT,
    user_id text NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    project_id text NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    revision integer NOT NULL CHECK (revision > 0),
    effective_from date NOT NULL,
    recurrence_start_date date NOT NULL,
    effective_at timestamptz NOT NULL DEFAULT now(),
    weekday smallint NOT NULL CHECK (weekday BETWEEN 0 AND 6),
    local_start time NOT NULL,
    local_end time NOT NULL,
    end_day_offset smallint NOT NULL DEFAULT 0 CHECK (end_day_offset BETWEEN 0 AND 1),
    timezone text NOT NULL REFERENCES timezone_master(timezone) ON DELETE RESTRICT,
    frequency text NOT NULL CHECK (frequency IN ('once', 'weekly')),
    superseded_at timestamptz,
    changed_by text NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT project_time_allocation_rule_revisions_rule_user_fk
        FOREIGN KEY (rule_id, user_id) REFERENCES project_time_allocation_rules(id, user_id) ON DELETE RESTRICT,
    CONSTRAINT project_time_allocation_rule_revisions_revision_key UNIQUE (rule_id, revision),
    CONSTRAINT project_time_allocation_rule_revisions_source_key UNIQUE (id, rule_id, user_id),
    CONSTRAINT project_time_allocation_rule_revisions_interval CHECK (
        (end_day_offset = 1 AND local_end <= local_start) OR
        (end_day_offset = 0 AND local_end > local_start)
    )
);
CREATE UNIQUE INDEX idx_project_time_allocation_rule_revisions_effective
    ON project_time_allocation_rule_revisions(rule_id, effective_at) WHERE superseded_at IS NULL;

CREATE TABLE project_time_allocations (
    id text PRIMARY KEY CHECK (id ~ '^[0-9ABCDEFGHJKMNPQRSTVWXYZ]{26}$'),
    rule_id text NOT NULL,
    source_revision_id text NOT NULL,
    user_id text NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    project_id text NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    occurrence_date date NOT NULL,
    start_at timestamptz NOT NULL,
    end_at timestamptz NOT NULL,
    timezone text NOT NULL REFERENCES timezone_master(timezone) ON DELETE RESTRICT,
    deleted_at timestamptz,
    revision integer NOT NULL DEFAULT 1 CHECK (revision > 0),
    changed_by text NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT project_time_allocations_rule_user_fk FOREIGN KEY (rule_id, user_id)
        REFERENCES project_time_allocation_rules(id, user_id) ON DELETE RESTRICT,
    CONSTRAINT project_time_allocations_source_revision_fk FOREIGN KEY (source_revision_id, rule_id, user_id)
        REFERENCES project_time_allocation_rule_revisions(id, rule_id, user_id) ON DELETE RESTRICT,
    CONSTRAINT project_time_allocations_end_after_start CHECK (end_at > start_at),
    CONSTRAINT project_time_allocations_rule_date_key UNIQUE (rule_id, occurrence_date)
);
CREATE INDEX idx_project_time_allocations_user_start
    ON project_time_allocations(user_id, start_at, end_at) WHERE deleted_at IS NULL;
CREATE INDEX idx_project_time_allocations_project_start
    ON project_time_allocations(project_id, start_at, end_at) WHERE deleted_at IS NULL;

CREATE TABLE project_time_allocation_exceptions (
    rule_id text NOT NULL,
    occurrence_date date NOT NULL,
    source_revision_id text NOT NULL,
    user_id text NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    project_id text NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    start_at timestamptz NOT NULL,
    end_at timestamptz NOT NULL,
    timezone text NOT NULL REFERENCES timezone_master(timezone) ON DELETE RESTRICT,
    deleted_at timestamptz,
    changed_by text NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (rule_id, occurrence_date),
    CONSTRAINT project_time_allocation_exceptions_rule_user_fk FOREIGN KEY (rule_id, user_id)
        REFERENCES project_time_allocation_rules(id, user_id) ON DELETE RESTRICT,
    CONSTRAINT project_time_allocation_exceptions_source_fk FOREIGN KEY (source_revision_id, rule_id, user_id)
        REFERENCES project_time_allocation_rule_revisions(id, rule_id, user_id) ON DELETE RESTRICT,
    CONSTRAINT project_time_allocation_exceptions_end_after_start CHECK (end_at > start_at)
);

CREATE TABLE project_time_allocation_pause_intervals (
    project_id text NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    rule_id text NOT NULL REFERENCES project_time_allocation_rules(id) ON DELETE RESTRICT,
    paused_at timestamptz NOT NULL,
    resumed_at timestamptz,
    PRIMARY KEY (project_id, rule_id, paused_at),
    CHECK (resumed_at IS NULL OR resumed_at >= paused_at)
);
CREATE UNIQUE INDEX idx_project_time_allocation_pause_active
    ON project_time_allocation_pause_intervals(project_id, rule_id) WHERE resumed_at IS NULL;
CREATE INDEX idx_project_time_allocation_pause_interval
    ON project_time_allocation_pause_intervals(rule_id, project_id, paused_at, resumed_at);

CREATE TABLE daily_active_time_exceptions (
    user_id text NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    local_date date NOT NULL,
    active_start_local timestamp NOT NULL,
    active_end_local timestamp NOT NULL,
    timezone text NOT NULL REFERENCES timezone_master(timezone) ON DELETE RESTRICT,
    revision integer NOT NULL DEFAULT 1 CHECK (revision > 0),
    changed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, local_date),
    CONSTRAINT daily_active_time_exceptions_ordered CHECK (active_end_local > active_start_local)
);

CREATE FUNCTION prepare_project_time_allocation_revision() RETURNS trigger LANGUAGE plpgsql AS $$
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
CREATE TRIGGER project_time_allocations_revision_before_write
    BEFORE INSERT OR UPDATE ON project_time_allocations
    FOR EACH ROW EXECUTE FUNCTION prepare_project_time_allocation_revision();

