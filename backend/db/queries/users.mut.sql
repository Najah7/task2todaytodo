-- name: CreateUser :one
INSERT INTO users (id, first_name, last_name, email, password)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, first_name, last_name, email, password, created_at, updated_at;

-- name: UpdateUser :one
UPDATE users
SET first_name = $2, last_name = $3, email = $4, password = $5, updated_at = now()
WHERE id = $1
RETURNING id, first_name, last_name, email, password, created_at, updated_at;
