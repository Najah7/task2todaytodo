package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/billing/dao"
)

type checkoutStripeFake struct {
	found     bool
	active    dao.Subscription
	findCalls int
}

func (*checkoutStripeFake) CreateCustomer(context.Context, string, string, string) (string, error) {
	return "cus_test", nil
}
func (*checkoutStripeFake) CreateCheckout(context.Context, string, string, string, string, string, string) (dao.CheckoutReservation, error) {
	return dao.CheckoutReservation{}, nil
}
func (s *checkoutStripeFake) FindCurrentSubscription(context.Context, string) (dao.Subscription, bool, error) {
	s.findCalls++
	return s.active, s.found, nil
}
func (*checkoutStripeFake) GetCheckout(context.Context, string) (dao.CheckoutReservation, error) {
	return dao.CheckoutReservation{}, errors.New("unexpected checkout lookup")
}

func TestStartCheckoutReconcilesStripeBeforeReturningReservedURL(t *testing.T) {
	repo := &billingRepoFake{account: dao.Account{CustomerID: "cus_test", Checkout: dao.CheckoutReservation{IdempotencyKey: "reserved", Plan: "pro", URL: "https://checkout.stripe.com/existing", ExpiresAt: testTimePointer(time.Now().Add(time.Hour))}, CheckoutPlan: "pro"}}
	stripe := &checkoutStripeFake{found: true, active: dao.Subscription{ID: "sub_new", CustomerID: "cus_test", Status: "active", Plan: dao.PlanPro}}
	uow := billingUOWFake{repos: billingReposFake{repo}}
	uc := NewStartCheckoutUseCase(repo, uow, stripe, true, "price_pro", "price_team", "https://app.example")

	url, err := uc.Execute(context.Background(), "user", "pro")
	if !errors.Is(err, ErrConflict) || url != "" {
		t.Fatalf("Execute() = %q, %v; want conflict and no URL", url, err)
	}
	if stripe.findCalls != 1 || len(repo.snapshots) != 1 {
		t.Fatalf("lookup/snapshot calls = %d/%d; want 1/1", stripe.findCalls, len(repo.snapshots))
	}
}

func TestStartCheckoutStopsWhenReservationConflicts(t *testing.T) {
	repo := &billingRepoFake{reserveErr: ErrConflict}
	stripe := &checkoutStripeFake{}
	uow := billingUOWFake{repos: billingReposFake{repo}}
	uc := NewStartCheckoutUseCase(repo, uow, stripe, true, "price_pro", "price_team", "https://app.example")
	if _, err := uc.Execute(context.Background(), "user", "team"); !errors.Is(err, ErrConflict) {
		t.Fatalf("Execute error = %v; want conflict", err)
	}
	if stripe.findCalls != 0 {
		t.Fatalf("Stripe lookup calls = %d; want none", stripe.findCalls)
	}
}
