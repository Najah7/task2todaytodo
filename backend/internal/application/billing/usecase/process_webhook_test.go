package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/billing/dao"
)

type webhookStripeFake struct {
	subscriptions []dao.Subscription
	retrieved     []string
	released      []string
}

func (s *webhookStripeFake) RetrieveSubscription(_ context.Context, id string) (dao.Subscription, error) {
	s.retrieved = append(s.retrieved, id)
	index := len(s.retrieved) - 1
	if index >= len(s.subscriptions) {
		index = len(s.subscriptions) - 1
	}
	return s.subscriptions[index], nil
}

func (s *webhookStripeFake) ReleaseSchedule(_ context.Context, id string) error {
	s.released = append(s.released, id)
	return nil
}

func TestProcessWebhookPersistsCompletedCheckoutSubscription(t *testing.T) {
	repo := &billingRepoFake{cursor: dao.SnapshotCursor{UserID: "user"}}
	stripe := &webhookStripeFake{subscriptions: []dao.Subscription{{
		ID: "sub_new", Created: 200, CustomerID: "cus_user", Status: "active", Plan: dao.PlanTeam, PriceID: "price_team",
	}}}
	uc := NewProcessWebhookUseCase(billingUOWFake{repos: billingReposFake{repo}}, stripe, true)

	err := uc.Execute(context.Background(), "evt_checkout", "checkout.session.completed", 210, map[string]any{"subscription": "sub_new"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(repo.snapshots) != 1 || repo.snapshots[0].ID != "sub_new" || repo.account.State.Plan != "team" {
		t.Fatalf("persisted snapshot = %#v; state = %#v", repo.snapshots, repo.account.State)
	}
}

func TestProcessWebhookIgnoresDuplicateEvent(t *testing.T) {
	repo := &billingRepoFake{cursor: dao.SnapshotCursor{UserID: "user"}}
	stripe := &webhookStripeFake{subscriptions: []dao.Subscription{{
		ID: "sub_new", Created: 200, CustomerID: "cus_user", Status: "active", Plan: dao.PlanPro, PriceID: "price_pro",
	}}}
	uc := NewProcessWebhookUseCase(billingUOWFake{repos: billingReposFake{repo}}, stripe, true)
	object := map[string]any{"subscription": "sub_new"}
	for range 2 {
		if err := uc.Execute(context.Background(), "evt_duplicate", "checkout.session.completed", 210, object); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
	}
	if len(repo.snapshots) != 1 {
		t.Fatalf("snapshots applied = %d; want 1", len(repo.snapshots))
	}
}

func TestProcessWebhookReleasesScheduleForPeriodEndCancellation(t *testing.T) {
	pendingAt := time.Date(2026, time.October, 13, 0, 0, 0, 0, time.UTC)
	repo := &billingRepoFake{cursor: dao.SnapshotCursor{UserID: "user", Plan: "team", PendingPlan: "pro", ScheduleID: "sched_downgrade", PendingPlanAt: &pendingAt}}
	stripe := &webhookStripeFake{subscriptions: []dao.Subscription{
		{ID: "sub", CustomerID: "cus_user", Status: "active", Plan: dao.PlanTeam, PriceID: "price_team", ScheduleID: "sched_downgrade", CancelAtPeriodEnd: true},
		{ID: "sub", CustomerID: "cus_user", Status: "active", Plan: dao.PlanTeam, PriceID: "price_team", CancelAtPeriodEnd: true},
	}}
	uc := NewProcessWebhookUseCase(billingUOWFake{repos: billingReposFake{repo}}, stripe, true)

	err := uc.Execute(context.Background(), "evt_cancel", "customer.subscription.updated", 300, map[string]any{"id": "sub"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(stripe.released) != 1 || stripe.released[0] != "sched_downgrade" || len(stripe.retrieved) != 2 {
		t.Fatalf("Stripe schedule/subscription calls = released %v, retrieved %v", stripe.released, stripe.retrieved)
	}
	if len(repo.snapshots) != 1 || repo.account.ScheduleID != "" || repo.account.State.PendingPlan != nil {
		t.Fatalf("cancellation snapshot did not clear downgrade: account=%#v snapshots=%#v", repo.account, repo.snapshots)
	}
}

func TestProcessWebhookIgnoresOlderSubscriptionSnapshot(t *testing.T) {
	repo := &billingRepoFake{cursor: dao.SnapshotCursor{
		UserID: "user", SubscriptionID: "sub_current", SubscriptionCreated: 200, LastEventCreated: 400, Plan: "team",
	}}
	stripe := &webhookStripeFake{subscriptions: []dao.Subscription{{
		ID: "sub_old", Created: 100, CustomerID: "cus_user", Status: "canceled", Plan: dao.PlanPro, PriceID: "price_pro",
	}}}
	uc := NewProcessWebhookUseCase(billingUOWFake{repos: billingReposFake{repo}}, stripe, true)

	err := uc.Execute(context.Background(), "evt_old", "customer.subscription.deleted", 500, map[string]any{"id": "sub_old"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(repo.snapshots) != 0 {
		t.Fatalf("stale subscription overwrote current state: %#v", repo.snapshots)
	}
}
