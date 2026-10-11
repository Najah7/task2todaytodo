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

-- name: GetTimezoneChoices :many
SELECT timezone FROM timezone_master ORDER BY timezone;

-- name: IsTimezoneAllowed :one
SELECT EXISTS (SELECT 1 FROM timezone_master WHERE timezone = sqlc.arg(timezone)::text);
