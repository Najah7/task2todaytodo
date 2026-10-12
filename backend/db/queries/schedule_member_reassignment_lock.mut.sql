-- name: LockScheduleRuleForMemberReassignment :one
SELECT rule.id
FROM schedule_rules rule
WHERE rule.id = sqlc.arg(rule_id)::text AND rule.deleted_at IS NULL
FOR UPDATE;
