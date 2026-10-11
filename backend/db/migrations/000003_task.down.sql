DROP TABLE IF EXISTS action_items;
DROP TABLE IF EXISTS task_revisions;
DROP TABLE IF EXISTS tasks;
DROP TABLE IF EXISTS frequency_master;
DROP FUNCTION IF EXISTS snapshot_task_revision();
DROP FUNCTION IF EXISTS prepare_task_revision();
DROP FUNCTION IF EXISTS task_has_permission(text, text, text, action);
