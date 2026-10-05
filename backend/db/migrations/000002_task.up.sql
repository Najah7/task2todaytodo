CREATE TABLE priority_master (
    priority text PRIMARY KEY CHECK (priority ~ '^[a-z][a-z0-9_]*$'),
    label text NOT NULL CHECK (btrim(label) <> ''),
    label_jp text NOT NULL CHECK (btrim(label_jp) <> ''),
    weight integer NOT NULL CHECK (weight BETWEEN 0 AND 100),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE task_status_master (
    status text PRIMARY KEY CHECK (status ~ '^[a-z][a-z0-9_]*$'),
    label text NOT NULL CHECK (btrim(label) <> ''),
    label_jp text NOT NULL CHECK (btrim(label_jp) <> ''),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE frequency_master (
    frequency text PRIMARY KEY CHECK (frequency ~ '^[a-z][a-z0-9_]*$'),
    label text NOT NULL CHECK (btrim(label) <> ''),
    label_jp text NOT NULL CHECK (btrim(label_jp) <> ''),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE project_type_master (
    type text PRIMARY KEY CHECK (type ~ '^[a-z][a-z0-9_]*$'),
    label text NOT NULL CHECK (btrim(label) <> ''),
    label_jp text NOT NULL CHECK (btrim(label_jp) <> ''),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE projects (
    id text PRIMARY KEY CHECK (id ~ '^[0-9ABCDEFGHJKMNPQRSTVWXYZ]{26}$'),
    user_id text NOT NULL REFERENCES users(id),
    type text NOT NULL DEFAULT 'other' REFERENCES project_type_master(type) ON DELETE RESTRICT,
    title text NOT NULL CHECK (btrim(title) <> ''),
    goal text,
    description text,
    priority text NOT NULL DEFAULT 'low' REFERENCES priority_master(priority) ON DELETE RESTRICT,
    start_date date,
    end_date date,
    revision integer NOT NULL DEFAULT 1 CHECK (revision > 0),
    deleted_at timestamptz,
    changed_by text NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (start_date IS NULL OR end_date IS NULL OR end_date >= start_date),
    UNIQUE (id, user_id)
);

CREATE TABLE tasks (
    id text PRIMARY KEY CHECK (id ~ '^[0-9ABCDEFGHJKMNPQRSTVWXYZ]{26}$'),
    user_id text NOT NULL REFERENCES users(id),
    project_id text,
    assignee_id text NOT NULL REFERENCES users(id),
    title text NOT NULL CHECK (btrim(title) <> ''),
    description text,
    due_date date,
    estimated_minutes integer CHECK (estimated_minutes IS NULL OR estimated_minutes >= 0),
    actual_minutes integer CHECK (actual_minutes IS NULL OR actual_minutes >= 0),
    priority text NOT NULL DEFAULT 'low' REFERENCES priority_master(priority) ON DELETE RESTRICT,
    status text NOT NULL DEFAULT 'open' REFERENCES task_status_master(status) ON DELETE RESTRICT,
    revision integer NOT NULL DEFAULT 1 CHECK (revision > 0),
    deleted_at timestamptz,
    changed_by text NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (project_id, user_id) REFERENCES projects(id, user_id) ON DELETE RESTRICT
);

CREATE TABLE project_members (
    project_id text NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id text NOT NULL REFERENCES roles(role_id) ON DELETE RESTRICT,
    added_by text NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, user_id)
);

CREATE TABLE project_revisions (
    id text NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    revision integer NOT NULL CHECK (revision > 0),
    user_id text NOT NULL REFERENCES users(id),
    type text NOT NULL,
    title text NOT NULL,
    goal text,
    description text,
    priority text NOT NULL,
    start_date date,
    end_date date,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    changed_by text NOT NULL REFERENCES users(id),
    changed_at timestamptz NOT NULL,
    PRIMARY KEY (id, revision)
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
    estimated_minutes integer,
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

CREATE FUNCTION prepare_project_revision() RETURNS trigger LANGUAGE plpgsql AS $$
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

CREATE FUNCTION snapshot_project_revision() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO project_revisions (
        id, revision, user_id, type, title, goal, description, priority,
        start_date, end_date, deleted_at, created_at, updated_at,
        changed_by, changed_at
    ) VALUES (
        NEW.id, NEW.revision, NEW.user_id, NEW.type, NEW.title, NEW.goal,
        NEW.description, NEW.priority, NEW.start_date, NEW.end_date,
        NEW.deleted_at, NEW.created_at, NEW.updated_at, NEW.changed_by, NEW.updated_at
    );
    RETURN NEW;
END;
$$;

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
        due_date, estimated_minutes, actual_minutes, priority, status,
        deleted_at, created_at, updated_at, changed_by, changed_at
    ) VALUES (
        NEW.id, NEW.revision, NEW.user_id, NEW.project_id, NEW.assignee_id,
        NEW.title, NEW.description, NEW.due_date, NEW.estimated_minutes,
        NEW.actual_minutes, NEW.priority, NEW.status, NEW.deleted_at,
        NEW.created_at, NEW.updated_at, NEW.changed_by, NEW.updated_at
    );
    RETURN NEW;
END;
$$;

CREATE TRIGGER projects_revision_before_update
BEFORE INSERT OR UPDATE ON projects FOR EACH ROW EXECUTE FUNCTION prepare_project_revision();
CREATE TRIGGER projects_revision_snapshot
AFTER INSERT OR UPDATE ON projects FOR EACH ROW EXECUTE FUNCTION snapshot_project_revision();
CREATE TRIGGER tasks_revision_before_update
BEFORE INSERT OR UPDATE ON tasks FOR EACH ROW EXECUTE FUNCTION prepare_task_revision();
CREATE TRIGGER tasks_revision_snapshot
AFTER INSERT OR UPDATE ON tasks FOR EACH ROW EXECUTE FUNCTION snapshot_task_revision();

CREATE FUNCTION project_has_permission(project_key text, actor_key text, resource_key text, action_key permission_action)
RETURNS boolean LANGUAGE sql STABLE AS $$
    SELECT EXISTS (
        SELECT 1
        FROM projects AS p
        WHERE p.id = project_key
          AND (
              p.user_id = actor_key
              OR EXISTS (
                  SELECT 1
                  FROM project_members AS pm
                  JOIN role_permissions AS rp ON rp.role_id = pm.role_id
                  JOIN permissions AS allow_rule ON allow_rule.permission_id = rp.permission_id
                  WHERE pm.project_id = p.id
                    AND pm.user_id = actor_key
                    AND allow_rule.resource_id = resource_key
                    AND allow_rule.action = action_key
                    AND allow_rule.effect = 'allow'
                    AND NOT EXISTS (
                        SELECT 1
                        FROM role_permissions AS deny_grant
                        JOIN permissions AS deny_rule ON deny_rule.permission_id = deny_grant.permission_id
                        WHERE deny_grant.role_id = pm.role_id
                          AND deny_rule.resource_id = resource_key
                          AND deny_rule.action = action_key
                          AND deny_rule.effect = 'deny'
                    )
              )
          )
    )
$$;

CREATE FUNCTION task_has_permission(task_key text, actor_key text, resource_key text, action_key permission_action)
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

CREATE TABLE task_tags (
    id text PRIMARY KEY CHECK (id ~ '^[0-9ABCDEFGHJKMNPQRSTVWXYZ]{26}$'),
    user_id text NOT NULL REFERENCES users(id),
    name citext NOT NULL CHECK (btrim(name::text) <> ''),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, name)
);

CREATE TABLE task_tag_assignments (
    task_id text NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    tag_id text NOT NULL REFERENCES task_tags(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (task_id, tag_id)
);

CREATE TABLE todo_lists (
    id text PRIMARY KEY CHECK (id ~ '^[0-9ABCDEFGHJKMNPQRSTVWXYZ]{26}$'),
    user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    list_date date NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, list_date)
);

CREATE TABLE task_schedules (
    id text PRIMARY KEY CHECK (id ~ '^[0-9ABCDEFGHJKMNPQRSTVWXYZ]{26}$'),
    task_id text NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
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
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (end_at > start_at),
    CONSTRAINT task_schedules_id_task_id_key UNIQUE (id, task_id),
    CONSTRAINT task_schedules_series_task_fk
        FOREIGN KEY (series_id, task_id) REFERENCES task_schedules(id, task_id) ON DELETE CASCADE,
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
CREATE UNIQUE INDEX idx_task_schedules_series_occurrence_child_key
    ON task_schedules(series_id, occurrence_date) WHERE id <> series_id;

CREATE TABLE task_schedule_frequencies (
    task_schedule_id text NOT NULL REFERENCES task_schedules(id) ON DELETE CASCADE,
    frequency text NOT NULL REFERENCES frequency_master(frequency) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (task_schedule_id, frequency)
);

CREATE TABLE todo_items (
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
    is_exception boolean NOT NULL DEFAULT false,
    repeat_state text DEFAULT 'one_off',
    frequency_anchor_date date,
    interval_weeks integer NOT NULL DEFAULT 0 CHECK (interval_weeks >= 0),
    deleted_at timestamptz,
    skipped_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT todo_items_id_task_id_key UNIQUE (id, task_id),
    CONSTRAINT todo_items_series_task_fk
        FOREIGN KEY (series_id, task_id) REFERENCES todo_items(id, task_id) ON DELETE CASCADE,
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
CREATE UNIQUE INDEX idx_todo_items_series_occurrence_child_key
    ON todo_items(series_id, occurrence_date) WHERE id <> series_id;

CREATE TABLE todo_item_frequencies (
    todo_item_id text NOT NULL REFERENCES todo_items(id) ON DELETE CASCADE,
    frequency text NOT NULL REFERENCES frequency_master(frequency) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (todo_item_id, frequency)
);

CREATE TABLE todo_list_items (
    todo_list_id text NOT NULL REFERENCES todo_lists(id) ON DELETE CASCADE,
    todo_item_id text NOT NULL REFERENCES todo_items(id) ON DELETE CASCADE,
    position integer NOT NULL CHECK (position >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (todo_list_id, todo_item_id),
    CONSTRAINT todo_list_items_todo_list_id_position_key
        UNIQUE (todo_list_id, position) DEFERRABLE INITIALLY IMMEDIATE
);

CREATE TABLE todo_list_task_schedules (
    todo_list_id text NOT NULL REFERENCES todo_lists(id) ON DELETE CASCADE,
    task_schedule_id text NOT NULL REFERENCES task_schedules(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (todo_list_id, task_schedule_id)
);



CREATE INDEX idx_projects_user_id ON projects(user_id);
CREATE INDEX idx_projects_user_id_type ON projects(user_id, type);
CREATE INDEX idx_projects_user_id_priority ON projects(user_id, priority);
CREATE INDEX idx_projects_user_created_id ON projects(user_id, created_at DESC, id DESC);
CREATE INDEX idx_project_members_user_project ON project_members(user_id, project_id);
CREATE INDEX idx_tasks_user_id ON tasks(user_id);
CREATE INDEX idx_tasks_assignee_created_id ON tasks(assignee_id, created_at DESC, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_tasks_project_id ON tasks(project_id);
CREATE INDEX idx_tasks_user_id_status ON tasks(user_id, status);
CREATE INDEX idx_tasks_user_id_priority ON tasks(user_id, priority);
CREATE INDEX idx_tasks_user_created_id ON tasks(user_id, created_at DESC, id DESC);
CREATE INDEX idx_tasks_project_user_created_id ON tasks(project_id, user_id, created_at DESC, id DESC);
CREATE INDEX idx_task_tags_user_name_id ON task_tags(user_id, name, id);
CREATE INDEX idx_todo_items_task_position_occurrence_id_live
    ON todo_items(task_id, position, occurrence_date, id) WHERE deleted_at IS NULL;
CREATE INDEX idx_task_schedules_task_start_id_live
    ON task_schedules(task_id, start_at, id) WHERE deleted_at IS NULL;
CREATE INDEX idx_project_revisions_changed_at ON project_revisions(id, changed_at DESC, revision DESC);
CREATE INDEX idx_task_revisions_changed_at ON task_revisions(id, changed_at DESC, revision DESC);
CREATE INDEX idx_task_tag_assignments_tag_id_task_id ON task_tag_assignments(tag_id, task_id);
CREATE INDEX idx_todo_list_items_item_id_list_id ON todo_list_items(todo_item_id, todo_list_id);
CREATE INDEX idx_todo_list_schedules_schedule_id_list_id ON todo_list_task_schedules(task_schedule_id, todo_list_id);

CREATE FUNCTION recurrence_wall_time_exists(local_time timestamp without time zone, timezone_name text)
RETURNS boolean
LANGUAGE sql
STABLE
STRICT
AS $$
    SELECT ((local_time AT TIME ZONE timezone_name) AT TIME ZONE timezone_name) = local_time
$$;

INSERT INTO project_type_master (type, label, label_jp) VALUES
    ('work', 'Work', '仕事'),
    ('side_work', 'Side work', '副業'),
    ('study', 'Study', '勉強'),
    ('book', 'Book', '読書'),
    ('personal_project', 'Personal Project', '個人プロジェクト'),
    ('hobby', 'Hobby', '趣味'),
    ('other', 'Other', 'その他');

INSERT INTO priority_master (priority, label, label_jp, weight) VALUES
    ('urgent', 'Urgent', '緊急', 100),
    ('high', 'High', '高', 50),
    ('medium', 'Medium', '中', 25),
    ('low', 'Low', '低', 10),
    ('someday', 'Someday', 'いつか', 0);

INSERT INTO task_status_master (status, label, label_jp) VALUES
    ('open', 'Open', 'オープン'),
    ('pending', 'Pending', '保留'),
    ('waiting_on_others', 'Waiting on others', '他者待ち'),
    ('in_progress', 'In progress', '進行中'),
    ('done', 'Done', '完了');

INSERT INTO frequency_master (frequency, label, label_jp) VALUES
    ('mon', 'Monday', '月曜日'),
    ('tue', 'Tuesday', '火曜日'),
    ('wed', 'Wednesday', '水曜日'),
    ('thu', 'Thursday', '木曜日'),
    ('fri', 'Friday', '金曜日'),
    ('sat', 'Saturday', '土曜日'),
    ('sun', 'Sunday', '日曜日');
