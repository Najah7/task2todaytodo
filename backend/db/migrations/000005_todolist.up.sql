CREATE TABLE todo_lists (
    id text PRIMARY KEY CHECK (id ~ '^[0-9ABCDEFGHJKMNPQRSTVWXYZ]{26}$'),
    user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    list_date date NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, list_date)
);

CREATE TABLE todo_list_items (
    todo_list_id text NOT NULL REFERENCES todo_lists(id) ON DELETE CASCADE,
    action_item_id text NOT NULL REFERENCES action_items(id) ON DELETE CASCADE,
    position integer NOT NULL CHECK (position >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (todo_list_id, action_item_id),
    CONSTRAINT todo_list_items_todo_list_id_position_key
        UNIQUE (todo_list_id, position) DEFERRABLE INITIALLY IMMEDIATE
);

CREATE INDEX idx_todo_list_items_action_item_id_list_id ON todo_list_items(action_item_id, todo_list_id);

CREATE TABLE todo_list_schedules (
    todo_list_id text NOT NULL REFERENCES todo_lists(id) ON DELETE CASCADE,
    schedule_id text NOT NULL REFERENCES schedules(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (todo_list_id, schedule_id)
);

CREATE INDEX idx_todo_list_schedules_schedule_id_list_id
    ON todo_list_schedules(schedule_id, todo_list_id);
