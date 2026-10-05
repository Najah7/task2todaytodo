DROP FUNCTION IF EXISTS recurrence_wall_time_exists(timestamp without time zone, text);
-- Remove occurrence links and frequencies before their legacy revision rows.
DROP TABLE IF EXISTS todo_list_task_schedules;
DROP TABLE IF EXISTS todo_list_items;
DROP TABLE IF EXISTS todo_item_frequencies;
DROP TABLE IF EXISTS task_schedule_frequencies;
-- These legacy tables may be absent on databases created from the current task
-- UP migration. Drop dependent rows before the occurrence root rows.
DROP TABLE IF EXISTS task_schedule_revisions;
DROP TABLE IF EXISTS todo_item_revisions;
DROP TABLE IF EXISTS task_schedule_series_rules;
DROP TABLE IF EXISTS todo_item_series_rules;
DROP TABLE IF EXISTS task_schedule_occurrence_tombstones;
DROP TABLE IF EXISTS todo_item_occurrence_tombstones;
DROP TABLE IF EXISTS task_recurrence_pauses;
DROP TABLE IF EXISTS task_tag_assignments;
DROP TABLE IF EXISTS task_tags;
DROP TABLE IF EXISTS task_revisions;
DROP TABLE IF EXISTS project_revisions;
DROP TABLE IF EXISTS project_members;
DROP TABLE IF EXISTS task_schedules;
DROP TABLE IF EXISTS todo_items;
DROP TABLE IF EXISTS todo_lists;
DROP TABLE IF EXISTS tasks;
DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS project_type_master;
DROP TABLE IF EXISTS frequency_master;
DROP TABLE IF EXISTS task_status_master;
DROP TABLE IF EXISTS priority_master;
DROP FUNCTION IF EXISTS snapshot_task_revision();
DROP FUNCTION IF EXISTS prepare_task_revision();
DROP FUNCTION IF EXISTS snapshot_project_revision();
DROP FUNCTION IF EXISTS prepare_project_revision();
DROP FUNCTION IF EXISTS task_has_permission(text, text, text, permission_action);
DROP FUNCTION IF EXISTS project_has_permission(text, text, text, permission_action);
