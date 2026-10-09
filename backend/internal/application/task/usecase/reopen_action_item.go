package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type reopenActionItemUOW interface {
	Do(ctx context.Context, fn func(context.Context, Repositories) error) error
}

type ReopenActionItemUseCase struct {
	uow    reopenActionItemUOW
	logger logging.Logger
}

func NewReopenActionItemUseCase(uow reopenActionItemUOW, logger logging.Logger) *ReopenActionItemUseCase {
	return &ReopenActionItemUseCase{logger: logging.OrNop(logger), uow: uow}
}

func (uc *ReopenActionItemUseCase) Execute(ctx context.Context, userID domain.UserID, taskID domain.TaskID, actionItemID domain.ActionItemID) (err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "ReopenActionItemUseCase.Execute", err) }()

	asOf := time.Now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withTaskProgressMutationForPermission(ctx, repos, userID, taskID, asOf, shared.ActionItemUpdate(), func() error {
			return repos.ActionItems().UncheckForOwnedTask(ctx, userID, taskID, actionItemID)
		})
	})
	if err == nil {
		logTaskStateChange(uc.logger, ctx, "action_item.reopen", userID, taskID, string(actionItemID))
	}
	return err
}

func (uc *ReopenActionItemUseCase) ExecuteOccurrence(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.ActionItemID, occurrenceDate string) (err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "ReopenActionItemUseCase.ExecuteOccurrence", err) }()

	asOf := time.Now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withTaskProgressMutationForPermissionAndState(ctx, repos, userID, taskID, asOf, shared.ActionItemUpdate(), func(_ dao.Task, projectBefore taskProjectMutationSnapshot) error {
			state, err := loadActionItemOccurrenceWithCapability(ctx, repos, userID, taskID, seriesID, occurrenceDate, asOf, shared.ActionItemUpdate())
			if err != nil {
				return err
			}
			if projectBefore.State.Status == "done" && !state.saved {
				return ErrOccurrenceInactive
			}
			if !actionItemIsRecurring(state.root) {
				return repos.ActionItems().UncheckForOwnedTask(ctx, userID, taskID, domain.ActionItemID(state.root.ID))
			}
			if state.current == nil || !state.current.Completed {
				return nil
			}
			return repos.ActionItems().UncheckForOwnedTask(ctx, userID, taskID, domain.ActionItemID(state.current.ID))
		})
	})
	if err == nil {
		logTaskStateChange(uc.logger, ctx, "action_item.reopen_occurrence", userID, taskID, string(seriesID))
	}
	return err
}
