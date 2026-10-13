package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/allocation/dao"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

type GetPlansUseCase struct {
	uow    UnitOfWork
	access shared.PlanAccessReader
}

func NewGetPlansUseCase(uow UnitOfWork, access ...shared.PlanAccessReader) *GetPlansUseCase {
	uc := &GetPlansUseCase{uow: uow}
	if len(access) > 0 {
		uc.access = access[0]
	}
	return uc
}

func (uc *GetPlansUseCase) Execute(ctx context.Context, userID string) (dao.PlanCollection, error) {
	if err := requirePaid(ctx, uc.access, userID); err != nil {
		return dao.PlanCollection{}, err
	}
	var result dao.PlanCollection
	err := uc.uow.Do(ctx, func(ctx context.Context, repo Repository) error {
		if err := repo.LockUser(ctx, userID); err != nil {
			return err
		}
		items, err := repo.ListPlans(ctx, userID)
		if err != nil {
			return err
		}
		result = dao.PlanCollection{Revision: plansRevision(userID, items), Items: items}
		return nil
	})
	return result, err
}
