package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/allocation/dao"
)

type GetPlansUseCase struct{ uow UnitOfWork }

func NewGetPlansUseCase(uow UnitOfWork) *GetPlansUseCase { return &GetPlansUseCase{uow: uow} }

func (uc *GetPlansUseCase) Execute(ctx context.Context, userID string) (dao.PlanCollection, error) {
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
