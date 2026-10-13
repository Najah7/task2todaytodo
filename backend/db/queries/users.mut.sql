-- name: UpdateUserNameByID :one
UPDATE users SET first_name = $2, last_name = $3, updated_at = now()
WHERE id = $1 RETURNING *;

-- name: UpdateUserEmailByID :one
UPDATE users SET email = $2, updated_at = now()
WHERE id = $1 RETURNING *;

-- name: UpdateUserTimezone :one
UPDATE users
SET timezone = $2, updated_at = now()
WHERE id = $1
RETURNING timezone;

-- name: UpdateUserActiveTimeSettings :exec
UPDATE users
SET active_start_time = sqlc.arg(active_start_time)::time,
    active_end_time = sqlc.arg(active_end_time)::time,
    updated_at = now()
WHERE id = sqlc.arg(user_id)::text;

-- name: UpdateUserMinimumGapMinutes :exec
UPDATE users
SET minimum_gap_minutes = sqlc.arg(minimum_gap_minutes)::integer,
    updated_at = now()
WHERE id = sqlc.arg(user_id)::text;
