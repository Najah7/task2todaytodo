package usecase

import (
	"context"
	"math"

	"github.com/Najah7/task2todaytodo/internal/application/allocation/dao"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

type PutPlansUseCase struct {
	repo   Repository
	uow    UnitOfWork
	access shared.PlanAccessReader
}

func NewPutPlansUseCase(repo Repository, uow UnitOfWork, access ...shared.PlanAccessReader) *PutPlansUseCase {
	uc := &PutPlansUseCase{repo: repo, uow: uow}
	if len(access) > 0 {
		uc.access = access[0]
	}
	return uc
}

func (uc *PutPlansUseCase) Execute(ctx context.Context, userID, expectedRevision string, items []dao.PlanItem) (dao.PlanCollection, error) {
	if err := requirePaid(ctx, uc.access, userID); err != nil {
		return dao.PlanCollection{}, err
	}
	if userID == "" || expectedRevision == "" || !validPlans(items) {
		return dao.PlanCollection{}, ErrInvalidRequest
	}
	current, err := uc.repo.ListPlans(ctx, userID)
	if err != nil {
		return dao.PlanCollection{}, err
	}
	projectIDs := make([]string, 0, len(current)+len(items))
	for _, item := range current {
		projectIDs = append(projectIDs, item.ProjectID)
	}
	for _, item := range items {
		projectIDs = append(projectIDs, item.ProjectID)
	}
	var result dao.PlanCollection
	err = uc.uow.Do(ctx, func(ctx context.Context, repo Repository) error {
		if err := repo.LockProjectsForMutation(ctx, userID, projectIDs); err != nil {
			return err
		}
		if err := repo.LockUser(ctx, userID); err != nil {
			return err
		}
		current, err := repo.ListPlans(ctx, userID)
		if err != nil {
			return err
		}
		if plansRevision(userID, current) != expectedRevision {
			return ErrPreconditionFailed
		}
		requested := make(map[string]bool, len(items))
		for _, item := range items {
			if _, err := repo.GetProjectStatus(ctx, userID, item.ProjectID); err != nil {
				return err
			}
			requested[item.ProjectID] = true
			if err := repo.SetPlan(ctx, userID, item.ProjectID, item.TargetMinutes); err != nil {
				return err
			}
		}
		for _, item := range current {
			if requested[item.ProjectID] {
				continue
			}
			if err := repo.DeletePlan(ctx, userID, item.ProjectID); err != nil {
				return err
			}
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

func validPlans(items []dao.PlanItem) bool {
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		if item.ProjectID == "" || item.TargetMinutes < 0 || int64(item.TargetMinutes) > math.MaxInt32 || seen[item.ProjectID] {
			return false
		}
		seen[item.ProjectID] = true
	}
	return true
}
