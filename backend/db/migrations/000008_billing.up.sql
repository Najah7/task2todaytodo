CREATE TABLE user_billing (
    user_id text PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    stripe_customer_id text UNIQUE,
    stripe_subscription_id text UNIQUE,
    stripe_subscription_schedule_id text,
    stripe_subscription_created_at bigint NOT NULL DEFAULT 0,
    checkout_idempotency_key text,
    checkout_plan text CHECK (checkout_plan IS NULL OR checkout_plan IN ('pro', 'team')),
    checkout_session_id text,
    checkout_url text,
    checkout_expires_at timestamptz,
    plan text CHECK (plan IS NULL OR plan IN ('pro', 'team')),
    stripe_price_id text,
    status text,
    current_period_start timestamptz,
    current_period_end timestamptz,
    cancel_at_period_end boolean NOT NULL DEFAULT false,
    pending_plan text CHECK (pending_plan IS NULL OR pending_plan IN ('pro', 'team')),
    pending_plan_at timestamptz,
    last_event_created bigint NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (pending_plan IS NOT NULL OR pending_plan_at IS NULL)
);

CREATE TABLE stripe_webhook_events (
    event_id text PRIMARY KEY,
    created_at bigint NOT NULL,
    received_at timestamptz NOT NULL DEFAULT now()
);
