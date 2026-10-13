DROP TRIGGER IF EXISTS project_time_allocations_revision_before_write ON project_time_allocations;
DROP FUNCTION IF EXISTS prepare_project_time_allocation_revision();
DROP TABLE IF EXISTS daily_active_time_exceptions;
DROP TABLE IF EXISTS project_time_allocation_pause_intervals;
DROP TABLE IF EXISTS project_time_allocation_exceptions;
DROP TABLE IF EXISTS project_time_allocations;
DROP TABLE IF EXISTS project_time_allocation_rule_revisions;
DROP TABLE IF EXISTS project_time_allocation_plans;
DROP TABLE IF EXISTS project_time_allocation_rules;
