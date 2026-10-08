DROP INDEX IF EXISTS idx_projects_status_end_date;

CREATE OR REPLACE FUNCTION snapshot_project_revision() RETURNS trigger LANGUAGE plpgsql AS $$
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

ALTER TABLE project_revisions DROP COLUMN status;
ALTER TABLE projects DROP COLUMN status;
ALTER TABLE status_master RENAME TO task_status_master;
