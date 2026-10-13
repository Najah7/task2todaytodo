package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/billing/dao"
	"github.com/Najah7/task2todaytodo/internal/application/billing/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ queries *sqlc.Queries }

func New(pool *pgxpool.Pool) *Repository           { return &Repository{queries: sqlc.New(pool)} }
func (r *Repository) WithTx(tx pgx.Tx) *Repository { return &Repository{queries: sqlc.New(tx)} }

func (r *Repository) GetAccount(ctx context.Context, userID string) (dao.Account, error) {
	record, err := r.queries.GetBillingAccount(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return dao.Account{}, usecase.ErrNotFound
	}
	if err != nil {
		return dao.Account{}, err
	}
	return account(record.StripeCustomerID, record.StripeSubscriptionID, record.StripeSubscriptionScheduleID,
		record.CheckoutIdempotencyKey, record.CheckoutPlan, record.CheckoutSessionID, record.CheckoutUrl, record.CheckoutExpiresAt,
		record.Plan, record.StripePriceID, record.Status, record.CurrentPeriodEnd, record.CancelAtPeriodEnd, record.PendingPlan, record.PendingPlanAt), nil
}

func (r *Repository) EnsureAccount(ctx context.Context, userID string) error {
	return r.queries.EnsureBillingAccount(ctx, userID)
}

func (r *Repository) GetAccountForUpdate(ctx context.Context, userID string) (dao.Account, error) {
	record, err := r.queries.GetBillingAccountForUpdate(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return dao.Account{}, usecase.ErrNotFound
	}
	if err != nil {
		return dao.Account{}, err
	}
	return account(record.StripeCustomerID, record.StripeSubscriptionID, record.StripeSubscriptionScheduleID,
		record.CheckoutIdempotencyKey, record.CheckoutPlan, record.CheckoutSessionID, record.CheckoutUrl, record.CheckoutExpiresAt,
		record.Plan, record.StripePriceID, record.Status, record.CurrentPeriodEnd, record.CancelAtPeriodEnd, record.PendingPlan, record.PendingPlanAt), nil
}

func (r *Repository) GetSnapshotCursorForUpdate(ctx context.Context, customerID string) (dao.SnapshotCursor, error) {
	record, err := r.queries.GetBillingAccountByCustomerForUpdate(ctx, pgtype.Text{String: customerID, Valid: true})
	if errors.Is(err, pgx.ErrNoRows) {
		return dao.SnapshotCursor{}, errors.New("Stripe customer is not associated with a local user")
	}
	if err != nil {
		return dao.SnapshotCursor{}, err
	}
	cursor := dao.SnapshotCursor{UserID: record.UserID, Plan: record.Plan.String, PendingPlan: record.PendingPlan.String,
		ScheduleID: record.StripeSubscriptionScheduleID.String, SubscriptionCreated: record.StripeSubscriptionCreatedAt,
		LastEventCreated: record.LastEventCreated, SubscriptionID: record.StripeSubscriptionID.String}
	if record.PendingPlanAt.Valid {
		cursor.PendingPlanAt = timePointer(record.PendingPlanAt.Time)
	}
	return cursor, nil
}

func (r *Repository) GetUserEmail(ctx context.Context, userID string) (string, error) {
	return r.queries.GetBillingUserEmail(ctx, userID)
}

func (r *Repository) SaveCustomer(ctx context.Context, userID, customerID string) error {
	_, err := r.queries.SaveStripeCustomer(ctx, sqlc.SaveStripeCustomerParams{UserID: userID, StripeCustomerID: text(customerID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return usecase.ErrConflict
	}
	return err
}

func (r *Repository) ReserveCheckout(ctx context.Context, userID, key, plan string) error {
	rows, err := r.queries.ReserveBillingCheckout(ctx, sqlc.ReserveBillingCheckoutParams{UserID: userID, CheckoutIdempotencyKey: text(key), CheckoutPlan: text(plan)})
	if err != nil {
		return err
	}
	if rows != 1 {
		return usecase.ErrConflict
	}
	return nil
}

func (r *Repository) SaveCheckout(ctx context.Context, userID, key, sessionID, url string, expires time.Time) error {
	rows, err := r.queries.SaveBillingCheckout(ctx, sqlc.SaveBillingCheckoutParams{UserID: userID, CheckoutIdempotencyKey: text(key),
		CheckoutSessionID: text(sessionID), CheckoutUrl: text(url), CheckoutExpiresAt: timestamptz(expires)})
	if err != nil {
		return err
	}
	if rows != 1 {
		return usecase.ErrConflict
	}
	return nil
}

func (r *Repository) SavePendingPlan(ctx context.Context, userID string, plan dao.Plan, effectiveAt time.Time, scheduleID string) error {
	return r.queries.SaveBillingPendingPlan(ctx, sqlc.SaveBillingPendingPlanParams{UserID: userID, PendingPlan: text(string(plan)), PendingPlanAt: timestamptz(effectiveAt), StripeSubscriptionScheduleID: text(scheduleID)})
}

func (r *Repository) ClearPendingPlan(ctx context.Context, userID string) error {
	return r.queries.ClearBillingPendingPlan(ctx, userID)
}

func (r *Repository) ClearExpiredCheckout(ctx context.Context, userID, sessionID string) error {
	return r.queries.ClearBillingCheckout(ctx, sqlc.ClearBillingCheckoutParams{UserID: userID, CheckoutSessionID: text(sessionID)})
}

func (r *Repository) ClearExpiredCheckoutBySession(ctx context.Context, sessionID string) error {
	return r.queries.ClearBillingCheckoutBySession(ctx, text(sessionID))
}

func (r *Repository) RecordWebhookEvent(ctx context.Context, eventID string, created int64) (bool, error) {
	_, err := r.queries.RecordStripeWebhookEvent(ctx, sqlc.RecordStripeWebhookEventParams{EventID: eventID, CreatedAt: created})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return true, err
}

func (r *Repository) ApplySnapshot(ctx context.Context, userID string, sub dao.Subscription, plan, pendingPlan, scheduleID string, pendingAt *time.Time, eventCreated int64) error {
	return r.queries.ApplyBillingSnapshot(ctx, sqlc.ApplyBillingSnapshotParams{
		UserID: userID, SubscriptionID: text(sub.ID), SubscriptionCreatedAt: sub.Created,
		ScheduleID: nullableText(scheduleID), Plan: nullableText(plan), PriceID: nullableText(sub.PriceID),
		Status: text(sub.Status), CurrentPeriodStart: nullableTime(sub.CurrentPeriodStart), CurrentPeriodEnd: nullableTime(sub.CurrentPeriodEnd),
		CancelAtPeriodEnd: sub.CancelAtPeriodEnd, PendingPlan: nullableText(pendingPlan), PendingPlanAt: nullableTime(pendingAt), EventCreated: eventCreated,
	})
}

func account(customerID, subscriptionID, scheduleID, checkoutKey, checkoutPlan, sessionID, checkoutURL pgtype.Text,
	checkoutExpires pgtype.Timestamptz,
	plan, priceID, status pgtype.Text, periodEnd pgtype.Timestamptz, cancelAtPeriodEnd bool, pendingPlan pgtype.Text, pendingAt pgtype.Timestamptz) dao.Account {
	result := dao.Account{CustomerID: customerID.String, SubscriptionID: subscriptionID.String, ScheduleID: scheduleID.String, PriceID: priceID.String,
		State: dao.State{Plan: plan.String, Status: status.String, CancelAtPeriodEnd: cancelAtPeriodEnd}}
	if result.State.Plan == "" {
		result.State.Plan = "free"
	}
	if result.State.Status == "" {
		result.State.Status = "none"
	}
	if periodEnd.Valid {
		result.State.CurrentPeriodEnd = timePointer(periodEnd.Time)
	}
	if pendingPlan.Valid {
		result.State.PendingPlan = stringPointer(pendingPlan.String)
	}
	if pendingAt.Valid {
		result.State.PendingPlanAt = timePointer(pendingAt.Time)
	}
	if checkoutKey.Valid {
		result.Checkout.IdempotencyKey = checkoutKey.String
		result.State.CheckoutPending = true
	}
	result.Checkout.Plan = checkoutPlan.String
	result.Checkout.SessionID = sessionID.String
	result.Checkout.URL = checkoutURL.String
	result.CheckoutPlan = checkoutPlan.String
	if checkoutExpires.Valid {
		result.Checkout.ExpiresAt = timePointer(checkoutExpires.Time)
	}
	return result
}

func text(value string) pgtype.Text         { return pgtype.Text{String: value, Valid: true} }
func nullableText(value string) pgtype.Text { return pgtype.Text{String: value, Valid: value != ""} }
func timestamptz(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}
func nullableTime(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return timestamptz(*value)
}
func timePointer(value time.Time) *time.Time { return &value }
func stringPointer(value string) *string     { return &value }
