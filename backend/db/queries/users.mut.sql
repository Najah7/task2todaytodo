-- name: CreateUser :one
INSERT INTO users (id, first_name, last_name, email, password)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateUser :one
UPDATE users
SET first_name = $2, last_name = $3, email = $4, password = $5, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateUserTimezone :one
UPDATE users
SET timezone = $2, updated_at = now()
WHERE id = $1
RETURNING timezone;

-- name: UpdateUserSleepSettings :exec
UPDATE users
SET wake_up_time = sqlc.arg(wake_up_time)::time,
    bedtime = sqlc.arg(bedtime)::time,
    updated_at = now()
WHERE id = sqlc.arg(user_id)::text;
