package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/billing/dao"
)

type PlanChangeRepository interface {
	SavePendingPlan(context.Context, string, dao.Plan, time.Time, string) error
	ClearPendingPlan(context.Context, string) error
	GetSnapshotCursorForUpdate(context.Context, string) (dao.SnapshotCursor, error)
	ApplySnapshot(context.Context, string, dao.Subscription, string, string, string, *time.Time, int64) error
}

type planReadRepository interface {
	GetAccount(context.Context, string) (dao.Account, error)
}

type changePlanStripe interface {
	UpgradeSubscription(context.Context, string, string, string) (dao.Subscription, error)
	ScheduleDowngrade(context.Context, string, string, string, string) (string, error)
	ReleaseSchedule(context.Context, string) error
}

type ChangePlanUseCase struct {
	repo        planReadRepository
	uow         UnitOfWork
	stripe      changePlanStripe
	enabled     bool
	proPriceID  string
	teamPriceID string
}

func NewChangePlanUseCase(repo planReadRepository, uow UnitOfWork, stripe changePlanStripe, enabled bool, proPriceID, teamPriceID string) *ChangePlanUseCase {
	return &ChangePlanUseCase{repo: repo, uow: uow, stripe: stripe, enabled: enabled, proPriceID: proPriceID, teamPriceID: teamPriceID}
}

func (u *ChangePlanUseCase) Execute(ctx context.Context, userID, target string) (dao.State, error) {
	if !u.enabled {
		return dao.State{}, ErrDisabled
	}
	targetPrice, err := u.priceFor(target)
	if err != nil {
		return dao.State{}, err
	}
	account, err := u.repo.GetAccount(ctx, userID)
	if err != nil {
		return dao.State{}, err
	}
	if account.SubscriptionID == "" || account.State.Status != "active" || account.State.CancelAtPeriodEnd {
		return dao.State{}, ErrConflict
	}
	if account.State.Plan == target {
		if account.ScheduleID != "" {
			if err := u.stripe.ReleaseSchedule(ctx, account.ScheduleID); err != nil {
				return account.State, err
			}
			if err := u.clearPendingPlan(ctx, userID); err != nil {
				return dao.State{}, err
			}
			return u.statusAfterChange(ctx, userID)
		}
		return account.State, nil
	}
	if account.ScheduleID == "" && account.State.PendingPlan != nil && *account.State.PendingPlan == target {
		return account.State, nil
	}
	if account.ScheduleID != "" {
		if err := u.stripe.ReleaseSchedule(ctx, account.ScheduleID); err != nil {
			return account.State, err
		}
		// Release the old downgrade locally before attempting the new plan
		// change. In particular, a failed upgrade invoice must not leave the
		// database claiming a downgrade is scheduled after Stripe released it.
		if err := u.clearPendingPlan(ctx, userID); err != nil {
			return dao.State{}, err
		}
		account.ScheduleID = ""
		account.State.PendingPlan = nil
		account.State.PendingPlanAt = nil
	}
	if account.State.Plan == string(dao.PlanPro) && target == string(dao.PlanTeam) {
		sub, err := u.stripe.UpgradeSubscription(ctx, account.SubscriptionID, account.PriceID, targetPrice)
		if err != nil {
			return account.State, err
		}
		if sub.Status == "active" && (sub.Plan == dao.PlanTeam || sub.PendingPlan != nil) {
			if err = u.applySnapshot(ctx, sub, 0); err != nil {
				return dao.State{}, err
			}
		}
		return u.statusAfterChange(ctx, userID)
	}
	if account.State.Plan == string(dao.PlanTeam) && target == string(dao.PlanPro) {
		if account.State.CurrentPeriodEnd == nil {
			return dao.State{}, ErrConflict
		}
		scheduleID, err := u.stripe.ScheduleDowngrade(ctx, account.SubscriptionID, account.PriceID, targetPrice, account.CustomerID)
		if err != nil {
			return account.State, err
		}
		if err := u.uow.Do(ctx, func(ctx context.Context, repositories Repositories) error {
			return repositories.PlanChange().SavePendingPlan(ctx, userID, dao.PlanPro, *account.State.CurrentPeriodEnd, scheduleID)
		}); err != nil {
			return dao.State{}, err
		}
		return u.statusAfterChange(ctx, userID)
	}
	return dao.State{}, ErrConflict
}

func (u *ChangePlanUseCase) clearPendingPlan(ctx context.Context, userID string) error {
	return u.uow.Do(ctx, func(ctx context.Context, repositories Repositories) error {
		return repositories.PlanChange().ClearPendingPlan(ctx, userID)
	})
}

func (u *ChangePlanUseCase) applySnapshot(ctx context.Context, sub dao.Subscription, created int64) error {
	return u.uow.Do(ctx, func(ctx context.Context, repositories Repositories) error {
		repo := repositories.PlanChange()
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

func (u *ChangePlanUseCase) statusAfterChange(ctx context.Context, userID string) (dao.State, error) {
	account, err := u.repo.GetAccount(ctx, userID)
	if err != nil {
		return dao.State{}, err
	}
	return account.State, nil
}

func (u *ChangePlanUseCase) priceFor(plan string) (string, error) {
	switch dao.Plan(plan) {
	case dao.PlanPro:
		return u.proPriceID, nil
	case dao.PlanTeam:
		return u.teamPriceID, nil
	default:
		return "", ErrInvalidPlan
	}
}
