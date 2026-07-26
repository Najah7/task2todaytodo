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
    progress smallint NOT NULL DEFAULT 0 CHECK (progress BETWEEN 0 AND 100),
    priority text NOT NULL DEFAULT 'low' REFERENCES priority_master(priority) ON DELETE RESTRICT,
    start_at timestamptz NOT NULL,
    end_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (end_at > start_at),
    UNIQUE (id, user_id)
);

CREATE TABLE tasks (
    id text PRIMARY KEY CHECK (id ~ '^[0-9ABCDEFGHJKMNPQRSTVWXYZ]{26}$'),
    user_id text NOT NULL REFERENCES users(id),
    project_id text,
    title text NOT NULL CHECK (btrim(title) <> ''),
    description text,
    estimated_minutes integer CHECK (estimated_minutes IS NULL OR estimated_minutes >= 0),
    actual_minutes integer CHECK (actual_minutes IS NULL OR actual_minutes >= 0),
    progress smallint NOT NULL DEFAULT 0 CHECK (progress BETWEEN 0 AND 100),
    priority text NOT NULL DEFAULT 'low' REFERENCES priority_master(priority) ON DELETE RESTRICT,
    status text NOT NULL DEFAULT 'open' REFERENCES task_status_master(status) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE SET NULL
);

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
    interval_weeks integer NOT NULL DEFAULT 1 CHECK (interval_weeks >= 0),
    start_at timestamptz NOT NULL,
    end_at timestamptz NOT NULL,
    due_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (end_at > start_at)
);

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
    completed boolean NOT NULL DEFAULT false,
    position integer NOT NULL CHECK (position >= 0),
    interval_weeks integer NOT NULL DEFAULT 1 CHECK (interval_weeks >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT todo_items_task_id_position_key
        UNIQUE (task_id, position) DEFERRABLE INITIALLY IMMEDIATE
);

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
CREATE INDEX idx_tasks_user_id ON tasks(user_id);
CREATE INDEX idx_tasks_project_id ON tasks(project_id);
CREATE INDEX idx_tasks_user_id_status ON tasks(user_id, status);
CREATE INDEX idx_tasks_user_id_priority ON tasks(user_id, priority);
CREATE INDEX idx_task_schedules_task_id_start_at ON task_schedules(task_id, start_at);
CREATE INDEX idx_task_tag_assignments_tag_id_task_id ON task_tag_assignments(tag_id, task_id);
CREATE INDEX idx_todo_list_items_item_id_list_id ON todo_list_items(todo_item_id, todo_list_id);
CREATE INDEX idx_todo_list_schedules_schedule_id_list_id ON todo_list_task_schedules(task_schedule_id, todo_list_id);

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
