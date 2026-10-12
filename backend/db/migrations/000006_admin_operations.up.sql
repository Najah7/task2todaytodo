CREATE TABLE admin_operations (
    id text PRIMARY KEY CHECK (id ~ '^[0-9ABCDEFGHJKMNPQRSTVWXYZ]{26}$'),
    job_name text NOT NULL CHECK (job_name IN ('backfill-schedules', 'prefill-allocations')),
    state text NOT NULL CHECK (state IN ('running', 'succeeded', 'failed')),
    requested_by text NOT NULL REFERENCES users(id),
    cutoff timestamptz NOT NULL,
    started_at timestamptz NOT NULL,
    finished_at timestamptz,
    heartbeat_at timestamptz NOT NULL,
    lease_expires_at timestamptz NOT NULL,
    lease_token text NOT NULL,
    processed_rules bigint NOT NULL DEFAULT 0 CHECK (processed_rules >= 0),
    generated_schedules bigint NOT NULL DEFAULT 0 CHECK (generated_schedules >= 0),
    generated_occurrences bigint NOT NULL DEFAULT 0 CHECK (generated_occurrences >= 0),
    response jsonb,
    error_code text,
    error_message text,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((state = 'running' AND finished_at IS NULL AND response IS NULL AND error_code IS NULL AND error_message IS NULL)
        OR (state = 'succeeded' AND finished_at IS NOT NULL AND response IS NOT NULL AND error_code IS NULL AND error_message IS NULL)
        OR (state = 'failed' AND finished_at IS NOT NULL AND response IS NULL AND error_code IS NOT NULL AND error_message IS NOT NULL))
);

CREATE UNIQUE INDEX admin_operations_one_active_job ON admin_operations(job_name) WHERE state = 'running';
