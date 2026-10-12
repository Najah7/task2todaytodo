-- name: GetUserByEmailForAuth :one
SELECT * FROM users WHERE email = $1 FOR UPDATE;

-- name: GetUserByIDForAuth :one
SELECT * FROM users WHERE id = $1 FOR UPDATE;

-- name: CreateUnverifiedUser :one
INSERT INTO users (id, first_name, last_name, email, password_hash)
VALUES ($1, '', '', $2, $3)
RETURNING *;

-- name: MarkUserEmailVerified :execrows
UPDATE users SET email_verified_at = now(), updated_at = now()
WHERE id = $1 AND email_verified_at IS NULL;

-- name: UpdateUserPasswordForReset :execrows
UPDATE users
SET password_hash = $2,
    email_verified_at = COALESCE(email_verified_at, now()),
    auth_version = auth_version + 1,
    updated_at = now()
WHERE id = $1;

-- name: RevokeAllUserPersonalAccessTokens :exec
UPDATE personal_access_tokens SET revoked_at = now()
WHERE user_id = $1 AND revoked_at IS NULL;

-- name: RevokeUserAuthLinkTokens :exec
UPDATE auth_link_tokens SET revoked_at = now()
WHERE user_id = $1 AND used_at IS NULL AND revoked_at IS NULL;

-- name: RevokeActiveUserAuthLinkTokensForPurpose :exec
UPDATE auth_link_tokens SET revoked_at = now()
WHERE user_id = $1 AND purpose = $2 AND used_at IS NULL AND revoked_at IS NULL;

-- name: CreateAuthLinkToken :exec
INSERT INTO auth_link_tokens (token_hash, user_id, purpose, expires_at)
VALUES ($1, $2, $3, $4);

-- name: GetAuthLinkTokenForUpdate :one
SELECT token_hash, user_id, purpose, expires_at, used_at, revoked_at
FROM auth_link_tokens
WHERE token_hash = $1
FOR UPDATE;

-- name: GetAuthLinkToken :one
SELECT token_hash, user_id, purpose, expires_at, used_at, revoked_at
FROM auth_link_tokens
WHERE token_hash = $1;

-- name: ConsumeAuthLinkToken :execrows
UPDATE auth_link_tokens SET used_at = now()
WHERE token_hash = $1
  AND used_at IS NULL
  AND revoked_at IS NULL
  AND expires_at > now();

-- name: LockEmailDeliveryAttempts :exec
SELECT pg_advisory_xact_lock(hashtextextended(encode(sqlc.arg(email_hash)::bytea, 'hex'), 0));

-- name: DeleteExpiredEmailDeliveryAttempts :exec
DELETE FROM email_delivery_attempts WHERE email_hash = $1 AND attempted_at <= $2;

-- name: CountRecentEmailDeliveryAttempts :one
SELECT count(*)::bigint FROM email_delivery_attempts
WHERE email_hash = $1 AND attempted_at > $2;

-- name: CreateEmailDeliveryAttempt :exec
INSERT INTO email_delivery_attempts (email_hash, attempted_at) VALUES ($1, $2);
