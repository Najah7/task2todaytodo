CREATE TABLE frequency_master (
    frequency text PRIMARY KEY CHECK (frequency ~ '^[a-z][a-z0-9_]*$'),
    label text NOT NULL CHECK (btrim(label) <> ''),
    label_jp text NOT NULL CHECK (btrim(label_jp) <> ''),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE tasks (
    id text PRIMARY KEY CHECK (id ~ '^[0-9ABCDEFGHJKMNPQRSTVWXYZ]{26}$'),
    user_id text NOT NULL REFERENCES users(id),
    project_id text,
    assignee_id text NOT NULL REFERENCES users(id),
    title text NOT NULL CHECK (btrim(title) <> ''),
    description text,
    due_date date,
    manual_estimated_minutes integer CHECK (manual_estimated_minutes IS NULL OR manual_estimated_minutes >= 0),
    actual_minutes integer CHECK (actual_minutes IS NULL OR actual_minutes >= 0),
    priority text NOT NULL DEFAULT 'low' REFERENCES priority_master(priority) ON DELETE RESTRICT,
    status text NOT NULL DEFAULT 'open' REFERENCES status_master(status) ON DELETE RESTRICT,
    revision integer NOT NULL DEFAULT 1 CHECK (revision > 0),
    deleted_at timestamptz,
    changed_by text NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (project_id, user_id) REFERENCES projects(id, user_id) ON DELETE RESTRICT
);

CREATE TABLE task_revisions (
    id text NOT NULL REFERENCES tasks(id) ON DELETE RESTRICT,
    revision integer NOT NULL CHECK (revision > 0),
    user_id text NOT NULL REFERENCES users(id),
    project_id text,
    assignee_id text NOT NULL REFERENCES users(id),
    title text NOT NULL,
    description text,
    due_date date,
    manual_estimated_minutes integer,
    actual_minutes integer,
    priority text NOT NULL,
    status text NOT NULL,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    changed_by text NOT NULL REFERENCES users(id),
    changed_at timestamptz NOT NULL,
    PRIMARY KEY (id, revision)
);

CREATE FUNCTION prepare_task_revision() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        NEW.revision := 1;
        IF NEW.assignee_id IS NULL THEN NEW.assignee_id := NEW.user_id; END IF;
        IF NEW.changed_by IS NULL THEN NEW.changed_by := NEW.user_id; END IF;
    ELSE
        NEW.revision := OLD.revision + 1;
        NEW.updated_at := now();
    END IF;
    RETURN NEW;
END;
$$;

CREATE FUNCTION snapshot_task_revision() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO task_revisions (
        id, revision, user_id, project_id, assignee_id, title, description,
        due_date, manual_estimated_minutes, actual_minutes, priority, status,
        deleted_at, created_at, updated_at, changed_by, changed_at
    ) VALUES (
        NEW.id, NEW.revision, NEW.user_id, NEW.project_id, NEW.assignee_id,
        NEW.title, NEW.description, NEW.due_date, NEW.manual_estimated_minutes,
        NEW.actual_minutes, NEW.priority, NEW.status, NEW.deleted_at,
        NEW.created_at, NEW.updated_at, NEW.changed_by, NEW.updated_at
    );
    RETURN NEW;
END;
$$;

CREATE TRIGGER tasks_revision_before_update
BEFORE INSERT OR UPDATE ON tasks FOR EACH ROW EXECUTE FUNCTION prepare_task_revision();
CREATE TRIGGER tasks_revision_snapshot
AFTER INSERT OR UPDATE ON tasks FOR EACH ROW EXECUTE FUNCTION snapshot_task_revision();

CREATE FUNCTION task_has_permission(task_key text, actor_key text, resource_key text, action_key action)
RETURNS boolean LANGUAGE sql STABLE AS $$
    SELECT EXISTS (
        SELECT 1
        FROM tasks AS t
        WHERE t.id = task_key
          AND t.deleted_at IS NULL
          AND (
              t.project_id IS NULL
              OR EXISTS (
                  SELECT 1 FROM projects AS parent
                  WHERE parent.id = t.project_id AND parent.deleted_at IS NULL
              )
          )
          AND (
              t.user_id = actor_key
              OR (t.project_id IS NOT NULL AND project_has_permission(t.project_id, actor_key, resource_key, action_key))
          )
    )
$$;

CREATE TABLE tags (
    id text PRIMARY KEY CHECK (id ~ '^[0-9ABCDEFGHJKMNPQRSTVWXYZ]{26}$'),
    user_id text NOT NULL REFERENCES users(id),
    name citext NOT NULL CHECK (btrim(name::text) <> ''),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, name)
);

CREATE TABLE task_tag_assignments (
    task_id text NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    tag_id text NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (task_id, tag_id)
);

CREATE TABLE action_items (
    id text PRIMARY KEY CHECK (id ~ '^[0-9ABCDEFGHJKMNPQRSTVWXYZ]{26}$'),
    task_id text NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    title text NOT NULL CHECK (btrim(title) <> ''),
    description text,
    due_date date,
    completed boolean NOT NULL DEFAULT false,
    position integer NOT NULL CHECK (position >= 0),
    series_id text NOT NULL,
    occurrence_date date NOT NULL,
    timezone text NOT NULL CHECK (btrim(timezone) <> ''),
    estimated_minutes integer CHECK (estimated_minutes IS NULL OR estimated_minutes >= 0),
    priority text NOT NULL DEFAULT 'low',
    is_exception boolean NOT NULL DEFAULT false,
    repeat_state text DEFAULT 'one_off',
    frequency_anchor_date date,
    interval_weeks integer NOT NULL DEFAULT 0 CHECK (interval_weeks >= 0),
    deleted_at timestamptz,
    skipped_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT action_items_priority_fk FOREIGN KEY (priority) REFERENCES priority_master(priority) ON DELETE RESTRICT,
    CONSTRAINT action_items_id_task_id_key UNIQUE (id, task_id),
    CONSTRAINT action_items_series_task_fk
        FOREIGN KEY (series_id, task_id) REFERENCES action_items(id, task_id) ON DELETE CASCADE,
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
CREATE UNIQUE INDEX idx_action_items_series_occurrence_child_key
    ON action_items(series_id, occurrence_date) WHERE id <> series_id;

CREATE TABLE action_item_frequencies (
    action_item_id text NOT NULL REFERENCES action_items(id) ON DELETE CASCADE,
    frequency text NOT NULL REFERENCES frequency_master(frequency) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (action_item_id, frequency)
);

CREATE FUNCTION recurrence_wall_time_exists(local_time timestamp without time zone, timezone_name text)
RETURNS boolean
LANGUAGE sql
STABLE
STRICT
AS $$
    SELECT ((local_time AT TIME ZONE timezone_name) AT TIME ZONE timezone_name) = local_time
$$;

INSERT INTO frequency_master (frequency, label, label_jp) VALUES
    ('mon', 'Monday', '月曜日'),
    ('tue', 'Tuesday', '火曜日'),
    ('wed', 'Wednesday', '水曜日'),
    ('thu', 'Thursday', '木曜日'),
    ('fri', 'Friday', '金曜日'),
    ('sat', 'Saturday', '土曜日'),
    ('sun', 'Sunday', '日曜日');

CREATE INDEX idx_tasks_user_id ON tasks(user_id);

CREATE INDEX idx_tasks_assignee_created_id ON tasks(assignee_id, created_at DESC, id DESC) WHERE deleted_at IS NULL;

CREATE INDEX idx_tasks_project_id ON tasks(project_id);

CREATE INDEX idx_tasks_user_id_status ON tasks(user_id, status);

CREATE INDEX idx_tasks_user_id_priority ON tasks(user_id, priority);

CREATE INDEX idx_tasks_user_created_id ON tasks(user_id, created_at DESC, id DESC);

CREATE INDEX idx_tasks_project_user_created_id ON tasks(project_id, user_id, created_at DESC, id DESC);

CREATE INDEX idx_tags_user_name_id ON tags(user_id, name, id);

CREATE INDEX idx_action_items_task_position_occurrence_id_live
    ON action_items(task_id, position, occurrence_date, id) WHERE deleted_at IS NULL;

CREATE INDEX idx_task_revisions_changed_at ON task_revisions(id, changed_at DESC, revision DESC);

CREATE INDEX idx_task_tag_assignments_tag_id_task_id ON task_tag_assignments(tag_id, task_id);
