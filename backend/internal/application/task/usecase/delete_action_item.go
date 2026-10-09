package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type deleteActionItemUOW interface {
	Do(ctx context.Context, fn func(context.Context, Repositories) error) error
}

type DeleteActionItemUseCase struct {
	uow    deleteActionItemUOW
	logger logging.Logger
}

func NewDeleteActionItemUseCase(uow deleteActionItemUOW, logger logging.Logger) *DeleteActionItemUseCase {
	return &DeleteActionItemUseCase{logger: logging.OrNop(logger), uow: uow}
}

func (uc *DeleteActionItemUseCase) Execute(ctx context.Context, userID domain.UserID, taskID domain.TaskID, actionItemID domain.ActionItemID) (err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "DeleteActionItemUseCase.Execute", err) }()

	asOf := time.Now()
	return uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withTaskProgressMutationForPermission(ctx, repos, userID, taskID, asOf, shared.ActionItemDelete(), func() error {
			actionItems := repos.ActionItems()
			item, err := getActionItemForCommand(ctx, actionItems, userID, taskID, actionItemID, shared.ActionItemDelete())
			if err != nil {
				return err
			}

			if item.RepeatState == repeatStateOneOff && item.Completed {
				return ErrOccurrenceCompleted
			}
			if item.Deleted {
				return ErrOccurrenceInactive
			}
			// Deleting the root suppresses future projections while retaining its
			// repeat metadata for saved occurrence history.
			if item.SeriesID == item.ID {
				if err := actionItems.TombstoneForOwnedTask(ctx, userID, taskID, actionItemID); err != nil {
					return err
				}
				return nil
			}

			// Keep the deleted occurrence row so this date stays suppressed.
			return actionItems.TombstoneForOwnedTask(ctx, userID, taskID, actionItemID)
		})
	})

}

func (uc *DeleteActionItemUseCase) DeleteSeries(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.ActionItemID) (err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "DeleteActionItemUseCase.DeleteSeries", err) }()

	return uc.Execute(ctx, userID, taskID, seriesID)

}
