package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

const maxFreeActiveProjects = 3

func planAccessFor(access shared.PlanAccessReader, ctx context.Context, userID domain.UserID) (shared.PlanAccess, error) {
	if access == nil {
		return shared.PlanAccessFree, nil
	}
	return access.GetPlanAccess(ctx, string(userID))
}

func enforceProjectCapacity(ctx context.Context, repo Repository, userID domain.UserID) error {
	if err := repo.LockUserForPlanLimit(ctx, userID); err != nil {
		return err
	}
	count, err := repo.CountActiveOwnedProjects(ctx, userID)
	if err != nil {
		return err
	}
	if count >= maxFreeActiveProjects {
		return ErrFreeProjectLimitReached
	}
	return nil
}
