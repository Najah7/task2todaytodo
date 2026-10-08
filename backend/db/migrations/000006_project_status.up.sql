ALTER TABLE task_status_master RENAME TO status_master;

ALTER TABLE projects
    ADD COLUMN status text NOT NULL DEFAULT 'open'
    REFERENCES status_master(status) ON DELETE RESTRICT;

ALTER TABLE project_revisions
    ADD COLUMN status text NOT NULL DEFAULT 'open';

CREATE OR REPLACE FUNCTION snapshot_project_revision() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO project_revisions (
        id, revision, user_id, type, title, goal, description, priority,
        start_date, end_date, status, deleted_at, created_at, updated_at,
        changed_by, changed_at
    ) VALUES (
        NEW.id, NEW.revision, NEW.user_id, NEW.type, NEW.title, NEW.goal,
        NEW.description, NEW.priority, NEW.start_date, NEW.end_date, NEW.status,
        NEW.deleted_at, NEW.created_at, NEW.updated_at, NEW.changed_by, NEW.updated_at
    );
    RETURN NEW;
END;
$$;

CREATE INDEX idx_projects_status_end_date ON projects(status, end_date, id) WHERE deleted_at IS NULL;
