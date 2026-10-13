-- name: EnsureBillingAccount :exec
INSERT INTO user_billing(user_id) VALUES ($1) ON CONFLICT (user_id) DO NOTHING;

-- name: GetBillingAccount :one
SELECT user_id, stripe_customer_id, stripe_subscription_id, stripe_subscription_schedule_id,
       stripe_subscription_created_at, checkout_idempotency_key, checkout_plan, checkout_session_id,
       checkout_url, checkout_expires_at, plan, stripe_price_id, status, current_period_start,
       current_period_end, cancel_at_period_end, pending_plan, pending_plan_at
FROM user_billing WHERE user_id = $1;

-- name: GetBillingAccountForUpdate :one
SELECT user_id, stripe_customer_id, stripe_subscription_id, stripe_subscription_schedule_id,
       stripe_subscription_created_at, checkout_idempotency_key, checkout_plan, checkout_session_id,
       checkout_url, checkout_expires_at, plan, stripe_price_id, status, current_period_start,
       current_period_end, cancel_at_period_end, pending_plan, pending_plan_at
FROM user_billing WHERE user_id = $1 FOR UPDATE;

-- name: GetBillingAccountByCustomerForUpdate :one
SELECT user_id, plan, pending_plan, stripe_subscription_schedule_id, pending_plan_at,
       stripe_subscription_created_at, last_event_created, stripe_subscription_id
FROM user_billing WHERE stripe_customer_id = $1 FOR UPDATE;

-- name: GetBillingUserEmail :one
SELECT email::text FROM users WHERE id = $1;

-- name: SaveStripeCustomer :one
INSERT INTO user_billing(user_id, stripe_customer_id) VALUES ($1, $2)
ON CONFLICT (user_id) DO UPDATE SET
    stripe_customer_id = COALESCE(user_billing.stripe_customer_id, EXCLUDED.stripe_customer_id), updated_at = now()
WHERE user_billing.stripe_customer_id IS NULL OR user_billing.stripe_customer_id = EXCLUDED.stripe_customer_id
RETURNING user_id;

-- name: ReserveBillingCheckout :execrows
UPDATE user_billing SET checkout_idempotency_key = $2, checkout_plan = $3,
    checkout_session_id = NULL, checkout_url = NULL, checkout_expires_at = NULL, updated_at = now()
WHERE user_id = $1;

-- name: SaveBillingCheckout :execrows
UPDATE user_billing SET checkout_session_id = $3, checkout_url = $4, checkout_expires_at = $5, updated_at = now()
WHERE user_id = $1 AND checkout_idempotency_key = $2;

-- name: SaveBillingPendingPlan :exec
UPDATE user_billing SET pending_plan = $2, pending_plan_at = $3,
    stripe_subscription_schedule_id = $4, updated_at = now() WHERE user_id = $1;

-- name: ClearBillingPendingPlan :exec
UPDATE user_billing SET pending_plan = NULL, pending_plan_at = NULL,
    stripe_subscription_schedule_id = NULL, updated_at = now() WHERE user_id = $1;

-- name: ClearBillingCheckout :exec
UPDATE user_billing SET checkout_idempotency_key = NULL, checkout_plan = NULL,
    checkout_session_id = NULL, checkout_url = NULL, checkout_expires_at = NULL, updated_at = now()
WHERE user_id = $1 AND checkout_session_id = $2;

-- name: ClearBillingCheckoutBySession :exec
UPDATE user_billing SET checkout_idempotency_key = NULL, checkout_plan = NULL,
    checkout_session_id = NULL, checkout_url = NULL, checkout_expires_at = NULL, updated_at = now()
WHERE checkout_session_id = $1;

-- name: RecordStripeWebhookEvent :one
INSERT INTO stripe_webhook_events(event_id, created_at) VALUES ($1, $2)
ON CONFLICT DO NOTHING RETURNING event_id;

-- name: ApplyBillingSnapshot :exec
UPDATE user_billing SET stripe_subscription_id = sqlc.arg(subscription_id),
    stripe_subscription_created_at = GREATEST(stripe_subscription_created_at, sqlc.arg(subscription_created_at)),
    stripe_subscription_schedule_id = sqlc.narg(schedule_id), plan = sqlc.narg(plan), stripe_price_id = sqlc.narg(price_id),
    status = sqlc.arg(status), current_period_start = sqlc.narg(current_period_start), current_period_end = sqlc.narg(current_period_end), cancel_at_period_end = sqlc.arg(cancel_at_period_end),
    pending_plan = sqlc.narg(pending_plan), pending_plan_at = sqlc.narg(pending_plan_at),
    checkout_idempotency_key = NULL, checkout_plan = NULL, checkout_session_id = NULL,
    checkout_url = NULL, checkout_expires_at = NULL,
    last_event_created = GREATEST(last_event_created, sqlc.arg(event_created)), updated_at = now()
WHERE user_id = sqlc.arg(user_id);
