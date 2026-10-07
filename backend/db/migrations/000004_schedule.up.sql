CREATE TABLE schedules (
    id text PRIMARY KEY CHECK (id ~ '^[0-9ABCDEFGHJKMNPQRSTVWXYZ]{26}$'),
    user_id text NOT NULL REFERENCES users(id),
    project_id text,
    assignee_id text NOT NULL REFERENCES users(id),
    title text NOT NULL CHECK (btrim(title) <> ''),
    description text,
    location text,
    start_at timestamptz NOT NULL,
    end_at timestamptz NOT NULL,
    series_id text NOT NULL,
    occurrence_date date NOT NULL,
    timezone text NOT NULL CHECK (btrim(timezone) <> ''),
    is_exception boolean NOT NULL DEFAULT false,
    repeat_state text DEFAULT 'one_off',
    frequency_anchor_date date,
    interval_weeks integer NOT NULL DEFAULT 0 CHECK (interval_weeks >= 0),
    completed boolean NOT NULL DEFAULT false,
    deleted_at timestamptz,
    skipped_at timestamptz,
    revision integer NOT NULL DEFAULT 1 CHECK (revision > 0),
    changed_by text NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (end_at > start_at),
    CONSTRAINT schedules_id_user_id_key UNIQUE (id, user_id),
    CONSTRAINT schedules_project_user_fk
        FOREIGN KEY (project_id, user_id) REFERENCES projects(id, user_id) ON DELETE RESTRICT,
    CONSTRAINT schedules_series_user_fk
        FOREIGN KEY (series_id, user_id) REFERENCES schedules(id, user_id) ON DELETE CASCADE,
    CHECK (repeat_state IS NULL OR repeat_state IN ('one_off', 'active', 'stopped')),
    CHECK (
        (id = series_id AND (
            (repeat_state = 'one_off' AND frequency_anchor_date IS NULL AND interval_weeks = 0) OR
            (repeat_state = 'active' AND frequency_anchor_date IS NOT NULL AND interval_weeks > 0) OR
            (repeat_state = 'stopped' AND frequency_anchor_date IS NOT NULL AND interval_weeks = 0)
        )) OR
        (id <> series_id AND repeat_state IS NULL AND frequency_anchor_date IS NULL AND interval_weeks = 0)
    )
);
CREATE UNIQUE INDEX idx_schedules_series_occurrence_child_key
    ON schedules(series_id, occurrence_date) WHERE id <> series_id;

CREATE TABLE schedule_frequencies (
    schedule_id text NOT NULL REFERENCES schedules(id) ON DELETE CASCADE,
    frequency text NOT NULL REFERENCES frequency_master(frequency) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (schedule_id, frequency)
);

CREATE TABLE schedule_revisions (
    id text NOT NULL REFERENCES schedules(id) ON DELETE RESTRICT,
    revision integer NOT NULL CHECK (revision > 0),
    user_id text NOT NULL REFERENCES users(id),
    project_id text,
    assignee_id text NOT NULL REFERENCES users(id),
    title text NOT NULL,
    description text,
    location text,
    start_at timestamptz NOT NULL,
    end_at timestamptz NOT NULL,
    series_id text NOT NULL,
    occurrence_date date NOT NULL,
    timezone text NOT NULL,
    is_exception boolean NOT NULL,
    repeat_state text,
    frequency_anchor_date date,
    interval_weeks integer NOT NULL,
    completed boolean NOT NULL,
    deleted_at timestamptz,
    skipped_at timestamptz,
    frequencies text[] NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    changed_by text NOT NULL REFERENCES users(id),
    changed_at timestamptz NOT NULL,
    PRIMARY KEY (id, revision)
);

CREATE FUNCTION prepare_schedule_revision() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        NEW.revision := 1;
        IF NEW.changed_by IS NULL THEN NEW.changed_by := NEW.user_id; END IF;
    ELSE
        NEW.revision := OLD.revision + 1;
        NEW.updated_at := now();
    END IF;
    RETURN NEW;
END;
$$;

CREATE FUNCTION snapshot_schedule_revision() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'INSERT' AND NEW.id = NEW.series_id AND NEW.repeat_state = 'active' THEN
        RETURN NEW;
    END IF;
    INSERT INTO schedule_revisions (
        id, revision, user_id, project_id, assignee_id, title, description, location,
        start_at, end_at, series_id, occurrence_date, timezone, is_exception,
        repeat_state, frequency_anchor_date, interval_weeks, completed,
        deleted_at, skipped_at, frequencies, created_at, updated_at, changed_by, changed_at
    ) VALUES (
        NEW.id, NEW.revision, NEW.user_id, NEW.project_id, NEW.assignee_id, NEW.title,
        NEW.description, NEW.location, NEW.start_at, NEW.end_at, NEW.series_id,
        NEW.occurrence_date, NEW.timezone, NEW.is_exception, NEW.repeat_state,
        NEW.frequency_anchor_date, NEW.interval_weeks, NEW.completed, NEW.deleted_at,
        NEW.skipped_at,
        ARRAY(SELECT f.frequency FROM schedule_frequencies f WHERE f.schedule_id = NEW.series_id ORDER BY f.frequency),
        NEW.created_at, NEW.updated_at, NEW.changed_by, NEW.updated_at
    );
    RETURN NEW;
END;
$$;

CREATE TRIGGER schedules_revision_before_update
BEFORE INSERT OR UPDATE ON schedules FOR EACH ROW EXECUTE FUNCTION prepare_schedule_revision();
CREATE TRIGGER schedules_revision_snapshot
AFTER INSERT OR UPDATE ON schedules FOR EACH ROW EXECUTE FUNCTION snapshot_schedule_revision();

CREATE INDEX idx_schedules_user_assignee_start_id_live
    ON schedules(user_id, assignee_id, start_at, id) WHERE deleted_at IS NULL;
CREATE INDEX idx_schedules_project_start_id_live
    ON schedules(project_id, start_at, id) WHERE deleted_at IS NULL;

CREATE FUNCTION schedule_has_permission(schedule_key text, actor_key text, resource_key text, action_key action)
RETURNS boolean LANGUAGE sql STABLE AS $$
    SELECT EXISTS (
        SELECT 1
        FROM schedules AS s
        WHERE s.id = schedule_key
          AND s.deleted_at IS NULL
          AND (
              s.project_id IS NULL
              OR EXISTS (
                  SELECT 1 FROM projects AS parent
                  WHERE parent.id = s.project_id AND parent.deleted_at IS NULL
              )
          )
          AND (
              s.user_id = actor_key
              OR (s.project_id IS NOT NULL AND project_has_permission(s.project_id, actor_key, resource_key, action_key))
          )
    )
$$;

CREATE TABLE schedule_tag_assignments (
    schedule_id text NOT NULL REFERENCES schedules(id) ON DELETE CASCADE,
    tag_id text NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (schedule_id, tag_id)
);

CREATE INDEX idx_schedule_tag_assignments_tag_id_schedule_id
    ON schedule_tag_assignments(tag_id, schedule_id);
