package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

const maxFreeActionItems = 100

type projectPlanLimitLifecycle interface {
	LockOwnerPlanLimit(context.Context, string) error
	LockUserPlanLimit(context.Context, string) error
}

func checkActionItemDestinationLimit(
	ctx context.Context,
	repos Repositories,
	access shared.PlanAccessReader,
	ownerID domain.UserID,
	taskID domain.TaskID,
	destinationProjectID string,
) error {
	if access != nil {
		plan, err := access.GetPlanAccess(ctx, string(ownerID))
		if err != nil {
			return err
		}
		if plan.IsPaid() {
			return nil
		}
	}

	lifecycle, ok := repos.ProjectLifecycle().(projectPlanLimitLifecycle)
	if !ok {
		return ErrProjectLifecycleUnavailable
	}
	var err error
	var destinationCount int
	if destinationProjectID != "" {
		if err = lifecycle.LockOwnerPlanLimit(ctx, destinationProjectID); err != nil {
			return err
		}
		destinationCount, err = repos.ActionItems().CountForProjectPlanLimit(ctx, destinationProjectID)
	} else {
		if err = lifecycle.LockUserPlanLimit(ctx, string(ownerID)); err != nil {
			return err
		}
		destinationCount, err = repos.ActionItems().CountProjectlessForUserPlanLimit(ctx, ownerID)
	}
	if err != nil {
		return err
	}
	taskCount, err := repos.ActionItems().CountForTaskPlanLimit(ctx, taskID)
	if err != nil {
		return err
	}
	if taskCount == 0 {
		return nil
	}
	if destinationCount+taskCount > maxFreeActionItems {
		return ErrFreeActionItemLimitReached
	}
	return nil
}

func checkNewActionItemLimit(ctx context.Context, repos Repositories, access shared.PlanAccessReader, task dao.Task) error {
	if access != nil {
		plan, err := access.GetPlanAccess(ctx, task.UserID)
		if err != nil {
			return err
		}
		if plan.IsPaid() {
			return nil
		}
	}
	lifecycle := repos.ProjectLifecycle()
	if lifecycle == nil {
		return ErrProjectLifecycleUnavailable
	}
	if task.ProjectID != "" {
		limits, ok := lifecycle.(projectPlanLimitLifecycle)
		if !ok {
			return ErrProjectLifecycleUnavailable
		}
		if err := limits.LockOwnerPlanLimit(ctx, task.ProjectID); err != nil {
			return err
		}
		count, err := repos.ActionItems().CountForProjectPlanLimit(ctx, task.ProjectID)
		if err != nil {
			return err
		}
		if count >= maxFreeActionItems {
			return ErrFreeActionItemLimitReached
		}
		return nil
	}
	limits, ok := lifecycle.(projectPlanLimitLifecycle)
	if !ok {
		return ErrProjectLifecycleUnavailable
	}
	if err := limits.LockUserPlanLimit(ctx, task.UserID); err != nil {
		return err
	}
	count, err := repos.ActionItems().CountProjectlessForUserPlanLimit(ctx, domain.UserID(task.UserID))
	if err != nil {
		return err
	}
	if count >= maxFreeActionItems {
		return ErrFreeActionItemLimitReached
	}
	return nil
}
