-- name: GetUser :one
SELECT id, first_name, last_name, email, password, created_at, updated_at
FROM users
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT id, first_name, last_name, email, password, created_at, updated_at
FROM users
WHERE email = $1;
