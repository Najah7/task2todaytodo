-- name: GetUser :one
SELECT *
FROM users
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT *
FROM users
WHERE email = $1;

-- name: GetUserTimezone :one
SELECT timezone
FROM users
WHERE id = $1;

-- name: GetUserActiveTimeSettings :one
SELECT to_char(active_start_time, 'HH24:MI') AS active_start_time,
       to_char(active_end_time, 'HH24:MI') AS active_end_time
FROM users
WHERE id = sqlc.arg(user_id)::text;

-- name: GetTimezoneChoices :many
SELECT timezone FROM timezone_master ORDER BY timezone;

-- name: IsTimezoneAllowed :one
SELECT EXISTS (SELECT 1 FROM timezone_master WHERE timezone = sqlc.arg(timezone)::text);
