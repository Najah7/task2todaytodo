-- name: GetPersonalAccessTokenByToken :one
SELECT token, user_id, expires_at, revoked_at, created_at
FROM personal_access_tokens
WHERE token = $1;
