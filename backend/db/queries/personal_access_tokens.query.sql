-- name: GetPersonalAccessTokenByToken :one
SELECT personal_access_tokens.token, personal_access_tokens.user_id,
       personal_access_tokens.expires_at, personal_access_tokens.revoked_at,
       personal_access_tokens.created_at
FROM personal_access_tokens
JOIN users ON users.id = personal_access_tokens.user_id
WHERE personal_access_tokens.token = $1
  AND users.email_verified_at IS NOT NULL
  AND personal_access_tokens.auth_version = users.auth_version;
