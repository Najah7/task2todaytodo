package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/billing/dao"
)

type checkoutReadRepository interface {
	GetAccount(context.Context, string) (dao.Account, error)
	GetUserEmail(context.Context, string) (string, error)
}

type CheckoutRepository interface {
	EnsureAccount(context.Context, string) error
	GetAccountForUpdate(context.Context, string) (dao.Account, error)
	ReserveCheckout(context.Context, string, string, string) error
	SaveCustomer(context.Context, string, string) error
	SaveCheckout(context.Context, string, string, string, string, time.Time) error
	ClearExpiredCheckout(context.Context, string, string) error
	GetSnapshotCursorForUpdate(context.Context, string) (dao.SnapshotCursor, error)
	ApplySnapshot(context.Context, string, dao.Subscription, string, string, string, *time.Time, int64) error
}
type checkoutUOW interface {
	Do(context.Context, func(context.Context, Repositories) error) error
}

type checkoutStripe interface {
	CreateCustomer(context.Context, string, string, string) (string, error)
	CreateCheckout(context.Context, string, string, string, string, string, string) (dao.CheckoutReservation, error)
	FindCurrentSubscription(context.Context, string) (dao.Subscription, bool, error)
	GetCheckout(context.Context, string) (dao.CheckoutReservation, error)
}

type StartCheckoutUseCase struct {
	repo        checkoutReadRepository
	uow         checkoutUOW
	stripe      checkoutStripe
	enabled     bool
	proPriceID  string
	teamPriceID string
	frontendURL string
}

func NewStartCheckoutUseCase(repo checkoutReadRepository, uow checkoutUOW, stripe checkoutStripe, enabled bool, proPriceID, teamPriceID, frontendURL string) *StartCheckoutUseCase {
	return &StartCheckoutUseCase{repo: repo, uow: uow, stripe: stripe, enabled: enabled, proPriceID: proPriceID, teamPriceID: teamPriceID, frontendURL: frontendURL}
}

func (u *StartCheckoutUseCase) Execute(ctx context.Context, userID, plan string) (string, error) {
	if !u.enabled {
		return "", ErrDisabled
	}
	priceID, err := u.priceFor(plan)
	if err != nil {
		return "", err
	}
	reservation := dao.CheckoutReservation{}
	err = u.uow.Do(ctx, func(ctx context.Context, repositories Repositories) error {
		repo := repositories.Checkout()
		if err := repo.EnsureAccount(ctx, userID); err != nil {
			return err
		}
		account, err := repo.GetAccountForUpdate(ctx, userID)
		if err != nil {
			return err
		}
		switch account.State.Status {
		case "active", "trialing", "past_due", "unpaid", "paused", "incomplete":
			return ErrConflict
		}
		if account.Checkout.IdempotencyKey != "" && (account.Checkout.ExpiresAt == nil || account.Checkout.ExpiresAt.After(time.Now())) {
			if account.CheckoutPlan != plan {
				return ErrConflict
			}
			reservation = account.Checkout
			return nil
		}
		key := newIdempotencyKey()
		if err := repo.ReserveCheckout(ctx, userID, key, plan); err != nil {
			return err
		}
		reservation = dao.CheckoutReservation{IdempotencyKey: key, Plan: plan}
		return nil
	})
	if err != nil {
		return "", err
	}
	if reservation.Plan != plan {
		return "", ErrConflict
	}
	account, accountErr := u.repo.GetAccount(ctx, userID)
	if accountErr != nil && !errors.Is(accountErr, ErrNotFound) {
		return "", accountErr
	}
	if account.CustomerID != "" {
		active, found, findErr := u.stripe.FindCurrentSubscription(ctx, account.CustomerID)
		if findErr != nil {
			return "", findErr
		}
		if found {
			if err := u.applySnapshot(ctx, active, 0); err != nil {
				return "", err
			}
			return "", ErrConflict
		}
	}
	if reservation.URL != "" && reservation.ExpiresAt != nil && reservation.ExpiresAt.After(time.Now()) {
		return reservation.URL, nil
	}
	if reservation.SessionID != "" {
		session, getErr := u.stripe.GetCheckout(ctx, reservation.SessionID)
		if getErr == nil && session.URL != "" && session.ExpiresAt != nil && session.ExpiresAt.After(time.Now()) {
			return session.URL, nil
		}
		if getErr == nil {
			_ = u.uow.Do(ctx, func(ctx context.Context, repositories Repositories) error {
				return repositories.Checkout().ClearExpiredCheckout(ctx, userID, reservation.SessionID)
			})
		}
		return "", ErrConflict
	}
	email, err := u.repo.GetUserEmail(ctx, userID)
	if err != nil {
		return "", err
	}
	customerID := account.CustomerID
	if customerID == "" {
		customerID, err = u.stripe.CreateCustomer(ctx, userID, email, "customer_"+userID)
		if err != nil {
			return "", err
		}
		if err = u.uow.Do(ctx, func(ctx context.Context, repositories Repositories) error {
			return repositories.Checkout().SaveCustomer(ctx, userID, customerID)
		}); err != nil {
			return "", err
		}
	}
	session, err := u.stripe.CreateCheckout(ctx, customerID, userID, priceID, u.frontendURL+"/billing?checkout=success", u.frontendURL+"/billing?checkout=cancelled", reservation.IdempotencyKey)
	if err != nil {
		return "", err
	}
	if session.URL == "" || session.ExpiresAt == nil {
		return "", errors.New("Stripe Checkout session was incomplete")
	}
	if err := u.uow.Do(ctx, func(ctx context.Context, repositories Repositories) error {
		return repositories.Checkout().SaveCheckout(ctx, userID, reservation.IdempotencyKey, session.SessionID, session.URL, *session.ExpiresAt)
	}); err != nil {
		return "", err
	}
	return session.URL, nil
}

func (u *StartCheckoutUseCase) applySnapshot(ctx context.Context, sub dao.Subscription, created int64) error {
	return u.uow.Do(ctx, func(ctx context.Context, repositories Repositories) error {
		repo := repositories.Checkout()
		cursor, err := repo.GetSnapshotCursorForUpdate(ctx, sub.CustomerID)
		if err != nil {
			return err
		}
		plan, pendingPlan, scheduleID, pendingAt, apply := snapshotValues(cursor, created, sub)
		if !apply {
			return nil
		}
		return repo.ApplySnapshot(ctx, cursor.UserID, sub, plan, pendingPlan, scheduleID, pendingAt, created)
	})
}

func (u *StartCheckoutUseCase) priceFor(plan string) (string, error) {
	switch dao.Plan(plan) {
	case dao.PlanPro:
		return u.proPriceID, nil
	case dao.PlanTeam:
		return u.teamPriceID, nil
	default:
		return "", ErrInvalidPlan
	}
}

func newIdempotencyKey() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return fmt.Sprintf("fallback_%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(value[:])
}
