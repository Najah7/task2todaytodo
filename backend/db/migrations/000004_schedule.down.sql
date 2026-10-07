DROP FUNCTION IF EXISTS schedule_has_permission(text, text, text, action);
DROP TRIGGER IF EXISTS schedules_revision_snapshot ON schedules;
DROP TRIGGER IF EXISTS schedules_revision_before_update ON schedules;
DROP FUNCTION IF EXISTS snapshot_schedule_revision();
DROP FUNCTION IF EXISTS prepare_schedule_revision();
DROP TABLE IF EXISTS schedule_revisions;
DROP TABLE IF EXISTS schedule_frequencies;
DROP TABLE IF EXISTS schedule_tag_assignments;
DROP TABLE IF EXISTS schedules;
