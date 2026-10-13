package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

type paidPlanAccess struct{}

func (paidPlanAccess) GetPlanAccess(context.Context, string) (shared.PlanAccess, error) {
	return shared.PlanAccessTeam, nil
}

func newPaidGetWeekUseCase(repo Repository) *GetWeekUseCase {
	return NewGetWeekUseCase(repo, paidPlanAccess{})
}
