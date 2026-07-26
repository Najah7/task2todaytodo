-- name: CreateAccessToken :one
INSERT INTO access_tokens (token, user_id, expires_at)
VALUES ($1, $2, $3)
RETURNING token, user_id, expires_at, revoked_at, created_at;

-- name: RevokeAccessToken :exec
UPDATE access_tokens
SET revoked_at = $2
WHERE token = $1;
