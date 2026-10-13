package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/billing/dao"
)

type changePlanStripeFake struct {
	result       dao.Subscription
	upgradeErr   error
	upgradeCalls int
	releasedIDs  []string
}

func (s *changePlanStripeFake) UpgradeSubscription(context.Context, string, string, string) (dao.Subscription, error) {
	s.upgradeCalls++
	return s.result, s.upgradeErr
}
func (*changePlanStripeFake) ScheduleDowngrade(context.Context, string, string, string, string) (string, error) {
	return "", nil
}
func (s *changePlanStripeFake) ReleaseSchedule(_ context.Context, id string) error {
	s.releasedIDs = append(s.releasedIDs, id)
	return nil
}

func TestChangePlanRetainsCurrentPlanUntilUpgradePaymentSucceeds(t *testing.T) {
	pending := dao.PlanTeam
	repo := &billingRepoFake{account: dao.Account{CustomerID: "cus_test", SubscriptionID: "sub_test", PriceID: "price_pro", State: dao.State{Plan: "pro", Status: "active"}}}
	uow := billingUOWFake{repos: billingReposFake{repo}}
	stripe := &changePlanStripeFake{result: dao.Subscription{ID: "sub_test", CustomerID: "cus_test", Status: "active", Plan: dao.PlanPro, PriceID: "price_pro", PendingPlan: &pending}}
	uc := NewChangePlanUseCase(repo, uow, stripe, true, "price_pro", "price_team")

	state, err := uc.Execute(context.Background(), "user", "team")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if state.Plan != "pro" || state.PendingPlan == nil || *state.PendingPlan != "team" {
		t.Fatalf("state = %#v; want Pro with Team pending", state)
	}
	if len(repo.snapshots) != 1 || repo.snapshots[0].Plan != dao.PlanPro {
		t.Fatalf("snapshots = %#v; want only unchanged Pro plan", repo.snapshots)
	}
}

func TestChangePlanClearsReleasedDowngradeWhenUpgradeFails(t *testing.T) {
	pending := "team"
	repo := &billingRepoFake{account: dao.Account{
		CustomerID:     "cus_test",
		SubscriptionID: "sub_test",
		ScheduleID:     "sched_test",
		PriceID:        "price_pro",
		State:          dao.State{Plan: "pro", Status: "active", PendingPlan: &pending},
	}}
	uow := billingUOWFake{repos: billingReposFake{repo}}
	stripe := &changePlanStripeFake{upgradeErr: errors.New("upgrade invoice failed")}
	uc := NewChangePlanUseCase(repo, uow, stripe, true, "price_pro", "price_team")

	state, err := uc.Execute(context.Background(), "user", "team")
	if err == nil {
		t.Fatal("Execute() error = nil; want failed upgrade")
	}
	if len(stripe.releasedIDs) != 1 || stripe.releasedIDs[0] != "sched_test" {
		t.Fatalf("released schedules = %v; want sched_test", stripe.releasedIDs)
	}
	if stripe.upgradeCalls != 1 {
		t.Fatalf("upgrade calls = %d; want 1", stripe.upgradeCalls)
	}
	if repo.clearedPendingPlan != 1 || repo.account.ScheduleID != "" || repo.account.State.PendingPlan != nil {
		t.Fatalf("local schedule state was not cleared: account=%#v clears=%d", repo.account, repo.clearedPendingPlan)
	}
	if state.PendingPlan != nil {
		t.Fatalf("returned stale pending plan: %#v", state)
	}
}
