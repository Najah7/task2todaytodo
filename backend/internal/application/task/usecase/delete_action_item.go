package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

var ErrActionItemCompleted = errors.New("completed action item cannot be deleted")

type DeleteActionItemUseCase struct {
	uow    UOW
	logger telemetry.Logger
}

func NewDeleteActionItemUseCase(uow UOW, logger telemetry.Logger) *DeleteActionItemUseCase {
	return &DeleteActionItemUseCase{logger: telemetry.OrNop(logger), uow: uow}
}

func (uc *DeleteActionItemUseCase) Execute(ctx context.Context, userID domain.UserID, taskID domain.TaskID, actionItemID domain.ActionItemID) (err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "DeleteActionItemUseCase.Execute", err) }()
	asOf := time.Now()
	return uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withTaskProgressMutationForPermission(ctx, repos, userID, taskID, asOf, shared.ActionItemDelete(), func() error {
			repository := repos.ActionItems()
			item, err := repository.GetForCommand(ctx, userID, taskID, actionItemID, shared.ActionItemDelete())
			if err != nil {
				return err
			}
			if item.Deleted {
				return ErrActionItemNotFound
			}
			if item.Completed {
				return ErrActionItemCompleted
			}
			return repository.DeleteForOwnedTask(ctx, userID, taskID, actionItemID)
		})
	})
}
