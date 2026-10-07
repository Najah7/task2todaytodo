DROP TABLE IF EXISTS project_revisions;
DROP TABLE IF EXISTS project_members;
DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS project_type_master;
DROP TABLE IF EXISTS priority_master;
DROP FUNCTION IF EXISTS snapshot_project_revision();
DROP FUNCTION IF EXISTS prepare_project_revision();
DROP FUNCTION IF EXISTS project_has_permission(text, text, text, action);
