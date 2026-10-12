CREATE TABLE todos (
    id text PRIMARY KEY CHECK (id ~ '^[0-9ABCDEFGHJKMNPQRSTVWXYZ]{26}$'),
    user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    todo_date date NOT NULL,
    timezone text NOT NULL REFERENCES timezone_master(timezone) ON DELETE RESTRICT,
    revision integer NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, todo_date)
);

CREATE TABLE todo_items (
    id text PRIMARY KEY CHECK (id ~ '^[0-9ABCDEFGHJKMNPQRSTVWXYZ]{26}$'),
    todo_id text NOT NULL REFERENCES todos(id) ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('task', 'action_item', 'schedule')),
    task_id text REFERENCES tasks(id) ON DELETE CASCADE,
    action_item_id text REFERENCES action_items(id) ON DELETE CASCADE,
    schedule_rule_id text REFERENCES schedule_rules(id) ON DELETE CASCADE,
    occurrence_date date,
    planned_minutes integer,
    scheduled_start timestamptz,
    scheduled_end timestamptz,
    completed boolean NOT NULL DEFAULT false,
    hidden boolean NOT NULL DEFAULT false,
    position integer NOT NULL CHECK (position >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT todo_items_schedule_time_pair CHECK (
        (scheduled_start IS NULL AND scheduled_end IS NULL)
        OR (scheduled_start IS NOT NULL AND scheduled_end IS NOT NULL AND scheduled_end > scheduled_start)
    ),
    CONSTRAINT todo_items_kind_source CHECK (
        (kind = 'task' AND task_id IS NOT NULL AND action_item_id IS NULL AND schedule_rule_id IS NULL
            AND occurrence_date IS NULL AND hidden = false
            AND planned_minutes IS NOT NULL AND planned_minutes > 0)
        OR (kind = 'action_item' AND task_id IS NULL AND action_item_id IS NOT NULL AND schedule_rule_id IS NULL
            AND occurrence_date IS NULL AND hidden = false
            AND planned_minutes IS NOT NULL AND planned_minutes > 0)
        OR (kind = 'schedule' AND task_id IS NULL AND action_item_id IS NULL AND schedule_rule_id IS NOT NULL
            AND occurrence_date IS NOT NULL AND scheduled_start IS NOT NULL AND scheduled_end IS NOT NULL
            AND planned_minutes IS NULL)
    ),
    CONSTRAINT todo_items_todo_position_key UNIQUE (todo_id, position) DEFERRABLE INITIALLY IMMEDIATE,
    CONSTRAINT todo_items_todo_schedule_key UNIQUE (todo_id, schedule_rule_id, occurrence_date)
);

CREATE INDEX idx_todo_items_todo_position_id ON todo_items(todo_id, position, id);
CREATE INDEX idx_todo_items_action_item_todo_id ON todo_items(action_item_id, todo_id)
    WHERE kind = 'action_item';
CREATE INDEX idx_todo_items_task_todo_id ON todo_items(task_id, todo_id)
    WHERE kind = 'task';
