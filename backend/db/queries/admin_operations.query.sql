-- name: AcquireAdminOperationLock :exec
SELECT pg_advisory_xact_lock(hashtext('admin/job'), hashtext(sqlc.arg(job_name)::text));

-- name: FailExpiredAdminOperations :exec
UPDATE admin_operations SET state='failed', finished_at=lease_expires_at,
    error_code='interrupted', error_message='Operation execution lease expired. Run job again to resume from saved progress.'
WHERE job_name=sqlc.arg(job_name)::text AND state='running' AND lease_expires_at <= now();

-- name: GetActiveAdminOperation :one
SELECT * FROM admin_operations WHERE job_name=sqlc.arg(job_name)::text AND state='running';

-- name: CreateAdminOperation :one
INSERT INTO admin_operations
    (id, job_name, state, requested_by, cutoff, started_at, heartbeat_at, lease_expires_at, lease_token)
VALUES (sqlc.arg(id)::text, sqlc.arg(job_name)::text, 'running', sqlc.arg(requested_by)::text,
    sqlc.arg(cutoff)::timestamptz, sqlc.arg(started_at)::timestamptz, now(), sqlc.arg(lease_expires_at)::timestamptz, sqlc.arg(lease_token)::text)
RETURNING *;

-- name: GetAdminOperation :one
SELECT * FROM admin_operations WHERE id=sqlc.arg(id)::text;

-- name: ListAdminOperations :many
SELECT * FROM admin_operations
WHERE sqlc.narg(cursor)::text IS NULL OR id < sqlc.narg(cursor)::text
ORDER BY id DESC LIMIT sqlc.arg(row_limit)::integer;

-- name: HeartbeatAdminOperation :execrows
UPDATE admin_operations SET heartbeat_at=now(), lease_expires_at=sqlc.arg(lease_expires_at)::timestamptz
WHERE id=sqlc.arg(id)::text AND lease_token=sqlc.arg(lease_token)::text
  AND state='running' AND lease_expires_at > now();

-- name: ProgressAdminOperation :execrows
UPDATE admin_operations SET heartbeat_at=now(), lease_expires_at=sqlc.arg(lease_expires_at)::timestamptz,
    processed_rules=sqlc.arg(processed_rules)::bigint,
    generated_schedules=CASE WHEN job_name='backfill-schedules' THEN sqlc.arg(generated_schedules)::bigint ELSE generated_schedules END,
    generated_occurrences=CASE WHEN job_name='prefill-allocations' THEN sqlc.arg(generated_schedules)::bigint ELSE generated_occurrences END
WHERE id=sqlc.arg(id)::text AND lease_token=sqlc.arg(lease_token)::text
  AND state='running' AND lease_expires_at > now();

-- name: SucceedAdminOperation :execrows
UPDATE admin_operations SET state='succeeded', finished_at=now(), response=sqlc.arg(response)::jsonb,
    processed_rules=sqlc.arg(processed_rules)::bigint,
    generated_schedules=CASE WHEN job_name='backfill-schedules' THEN sqlc.arg(generated_schedules)::bigint ELSE generated_schedules END,
    generated_occurrences=CASE WHEN job_name='prefill-allocations' THEN sqlc.arg(generated_schedules)::bigint ELSE generated_occurrences END,
    heartbeat_at=now()
WHERE id=sqlc.arg(id)::text AND lease_token=sqlc.arg(lease_token)::text
  AND state='running' AND lease_expires_at > now();

-- name: FailAdminOperation :execrows
UPDATE admin_operations SET state='failed', finished_at=now(), error_code=sqlc.arg(error_code)::text,
    error_message=sqlc.arg(error_message)::text
WHERE id=sqlc.arg(id)::text AND lease_token=sqlc.arg(lease_token)::text
  AND state='running' AND lease_expires_at > now();
