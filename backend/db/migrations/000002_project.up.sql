CREATE TABLE priority_master (
    priority text PRIMARY KEY CHECK (priority ~ '^[a-z][a-z0-9_]*$'),
    label text NOT NULL CHECK (btrim(label) <> ''),
    label_jp text NOT NULL CHECK (btrim(label_jp) <> ''),
    weight integer NOT NULL CHECK (weight BETWEEN 0 AND 100),
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

CREATE TRIGGER projects_revision_before_update
BEFORE INSERT OR UPDATE ON projects FOR EACH ROW EXECUTE FUNCTION prepare_project_revision();
CREATE TRIGGER projects_revision_snapshot
AFTER INSERT OR UPDATE ON projects FOR EACH ROW EXECUTE FUNCTION snapshot_project_revision();

CREATE FUNCTION project_has_permission(project_key text, actor_key text, resource_key text, action_key action)
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

CREATE INDEX idx_projects_user_id ON projects(user_id);

CREATE INDEX idx_projects_user_id_type ON projects(user_id, type);

CREATE INDEX idx_projects_user_id_priority ON projects(user_id, priority);

CREATE INDEX idx_projects_user_created_id ON projects(user_id, created_at DESC, id DESC);

CREATE INDEX idx_project_members_user_project ON project_members(user_id, project_id);

CREATE INDEX idx_project_revisions_changed_at ON project_revisions(id, changed_at DESC, revision DESC);
