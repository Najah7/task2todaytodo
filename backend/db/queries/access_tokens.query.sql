-- name: GetAccessTokenByToken :one
SELECT token, user_id, expires_at, revoked_at, created_at
FROM access_tokens
WHERE token = $1;
