package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/billing/dao"
)

type billingRepoFake struct {
	account                dao.Account
	reserveErr             error
	snapshots              []dao.Subscription
	cursor                 dao.SnapshotCursor
	seenEvents             map[string]bool
	recordedEvents         []string
	clearedCheckoutSession string
	clearedPendingPlan     int
}

func (r *billingRepoFake) GetAccount(context.Context, string) (dao.Account, error) {
	return r.account, nil
}
func (*billingRepoFake) GetUserEmail(context.Context, string) (string, error) {
	return "user@example.com", nil
}
func (*billingRepoFake) EnsureAccount(context.Context, string) error { return nil }
func (r *billingRepoFake) GetAccountForUpdate(context.Context, string) (dao.Account, error) {
	return r.account, nil
}
func (r *billingRepoFake) ReserveCheckout(_ context.Context, _, _, plan string) error {
	if r.reserveErr != nil {
		return r.reserveErr
	}
	r.account.Checkout.IdempotencyKey = "idempotency"
	r.account.CheckoutPlan = plan
	r.account.Checkout.Plan = plan
	return nil
}
func (*billingRepoFake) SaveCustomer(context.Context, string, string) error { return nil }
func (*billingRepoFake) SaveCheckout(context.Context, string, string, string, string, time.Time) error {
	return nil
}
func (*billingRepoFake) ClearExpiredCheckout(context.Context, string, string) error { return nil }
func (r *billingRepoFake) GetSnapshotCursorForUpdate(context.Context, string) (dao.SnapshotCursor, error) {
	if r.cursor.UserID == "" {
		r.cursor.UserID = "user"
	}
	return r.cursor, nil
}
func (r *billingRepoFake) ApplySnapshot(_ context.Context, _ string, sub dao.Subscription, plan, pending, _ string, _ *time.Time, _ int64) error {
	r.snapshots = append(r.snapshots, sub)
	r.account.State.Plan = plan
	if pending == "" {
		r.account.State.PendingPlan = nil
	} else {
		r.account.State.PendingPlan = &pending
	}
	return nil
}
func (r *billingRepoFake) SavePendingPlan(_ context.Context, _ string, plan dao.Plan, effectiveAt time.Time, _ string) error {
	value := string(plan)
	r.account.State.PendingPlan = &value
	r.account.State.PendingPlanAt = &effectiveAt
	return nil
}
func (r *billingRepoFake) ClearPendingPlan(context.Context, string) error {
	r.account.State.PendingPlan = nil
	r.account.State.PendingPlanAt = nil
	r.account.ScheduleID = ""
	r.clearedPendingPlan++
	return nil
}
func (r *billingRepoFake) RecordWebhookEvent(_ context.Context, eventID string, _ int64) (bool, error) {
	if r.seenEvents == nil {
		r.seenEvents = make(map[string]bool)
	}
	r.recordedEvents = append(r.recordedEvents, eventID)
	if r.seenEvents[eventID] {
		return false, nil
	}
	r.seenEvents[eventID] = true
	return true, nil
}
func (r *billingRepoFake) ClearExpiredCheckoutBySession(_ context.Context, sessionID string) error {
	r.clearedCheckoutSession = sessionID
	return nil
}

type billingReposFake struct{ repo *billingRepoFake }

func (r billingReposFake) Checkout() CheckoutRepository     { return r.repo }
func (r billingReposFake) Webhook() WebhookRepository       { return r.repo }
func (r billingReposFake) PlanChange() PlanChangeRepository { return r.repo }

type billingUOWFake struct{ repos Repositories }

func (u billingUOWFake) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	return fn(ctx, u.repos)
}

func testTimePointer(value time.Time) *time.Time { return &value }
