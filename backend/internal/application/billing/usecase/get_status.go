package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/billing/dao"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

type billingAccountReader interface {
	GetAccount(context.Context, string) (dao.Account, error)
}

type GetStatusUseCase struct {
	repo    billingAccountReader
	enabled bool
}

func NewGetStatusUseCase(repo billingAccountReader, enabled bool) *GetStatusUseCase {
	return &GetStatusUseCase{repo: repo, enabled: enabled}
}

func (u *GetStatusUseCase) Execute(ctx context.Context, userID string) (dao.State, error) {
	if !u.enabled {
		return dao.State{Plan: "free", Status: "none", AccessPlan: shared.PlanAccessFree}, nil
	}
	account, err := u.repo.GetAccount(ctx, userID)
	if errors.Is(err, ErrNotFound) {
		return dao.State{Plan: "free", Status: "none", AccessPlan: shared.PlanAccessFree}, nil
	}
	if err != nil {
		return dao.State{}, err
	}
	account.State.AccessPlan = effectivePlan(account.State, time.Now())
	return account.State, nil
}

func (u *GetStatusUseCase) GetPlanAccess(ctx context.Context, userID string) (shared.PlanAccess, error) {
	if !u.enabled {
		return shared.PlanAccessFree, nil
	}
	account, err := u.repo.GetAccount(ctx, userID)
	if errors.Is(err, ErrNotFound) {
		return shared.PlanAccessFree, nil
	}
	if err != nil {
		return shared.PlanAccessFree, err
	}
	return effectivePlan(account.State, time.Now()), nil
}

func effectivePlan(state dao.State, now time.Time) shared.PlanAccess {
	return shared.EffectivePlanAccess(state.Plan, state.Status, state.CurrentPeriodEnd, now)
}
