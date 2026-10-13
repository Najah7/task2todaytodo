# Billing guide

Follow the [application guide](../AGENTS.md). Routes are registered in [cmd/api/main.go](../../../cmd/api/main.go); Stripe HTTP and signature adapters live in [port/adapter/stripe](../../port/adapter/stripe).

## Features

- Persist each user's single Stripe Customer and subscription state, Checkout reservation, pending plan change, and processed webhook IDs.
- `usecase/` has one operation type per file: status, Checkout, Customer Portal, plan change, and webhook processing.
- `dao/` owns persisted read models; `repository/` maps generated sqlc methods to usecase ports. Query source is in [db/queries/billing.query.sql](../../../db/queries/billing.query.sql), generated output is in [db/sqlc/billing.query.sql.go](../../../db/sqlc/billing.query.sql.go), and the application root owns transactions in [billing_uow.go](../../billing_uow.go).
- Stripe calls use only configured Price IDs. Checkout quantity is one. Pro-to-Team uses an invoice with pending updates; Team-to-Pro uses a renewal schedule.
- Billing exposes the effective access plan from the persisted Stripe snapshot. Feature use cases enforce entitlements at their boundaries; Checkout reservations alone do not grant access, and active access ends at `current_period_end` even if cancellation is scheduled for that boundary.

## Endpoints

- `GET /api/billing`
- `POST /api/billing/checkout`, `/api/billing/plan`, `/api/billing/portal`
- Public signed webhook: `POST /api/webhooks/stripe`

## Change checks

- Stripe webhooks must use the raw body, verify signatures, record event IDs idempotently, and reconcile current subscription state. Stale events must not replace a newer subscription.
- Checkout reservations bind the selected plan and prevent duplicate active subscriptions, including when local webhook state lags Stripe.
- Portal cancellation is configured to end at the paid period boundary; if a scheduled downgrade is attached, release it before applying cancellation state.
- Never log credentials, signing secrets, or full Stripe IDs.

## Verification

- Usecase and signature-verification tests run without PostgreSQL. Repository invariants require an integration database for end-to-end verification.
