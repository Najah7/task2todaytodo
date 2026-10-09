package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type completeActionItemUOW interface {
	Do(ctx context.Context, fn func(context.Context, Repositories) error) error
}

type CompleteActionItemUseCase struct {
	uow    completeActionItemUOW
	ID     shared.ID
	logger logging.Logger
}

func NewCompleteActionItemUseCase(uow completeActionItemUOW, logger logging.Logger, ids ...shared.ID) *CompleteActionItemUseCase {
	var ID shared.ID
	if len(ids) > 0 {
		ID = ids[0]
	}
	return &CompleteActionItemUseCase{logger: logging.OrNop(logger), uow: uow, ID: ID}
}

func (uc *CompleteActionItemUseCase) ExecuteOccurrence(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.ActionItemID, occurrenceDate string) (err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "CompleteActionItemUseCase.ExecuteOccurrence", err) }()

	asOf := time.Now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withTaskProgressMutationForPermissionAndState(ctx, repos, userID, taskID, asOf, shared.ActionItemUpdate(), func(_ dao.Task, projectBefore taskProjectMutationSnapshot) error {
			state, err := loadActionItemOccurrenceWithCapability(ctx, repos, userID, taskID, seriesID, occurrenceDate, asOf, shared.ActionItemUpdate())
			if err != nil {
				return err
			}
			state.projectDone = projectBefore.State.Status == "done"
			if projectDoneSuppressesActionItemOccurrence(state) {
				return ErrOccurrenceInactive
			}
			if !actionItemIsRecurring(state.root) {
				return repos.ActionItems().CheckForOwnedTask(ctx, userID, taskID, domain.ActionItemID(state.root.ID))
			}
			if state.current != nil {
				return repos.ActionItems().CheckForOwnedTask(ctx, userID, taskID, domain.ActionItemID(state.current.ID))
			}
			id, err := generatedActionItemID(uc.ID)
			if err != nil {
				return err
			}
			item, err := actionItemDomainOccurrence(state.root, nil, state.date, id, true, asOf)
			if err != nil {
				return err
			}
			writer, err := requireActionItemOverride(repos.ActionItems())
			if err != nil {
				return err
			}
			_, err = writer.UpsertActionItemOverride(ctx, userID, item)
			return err
		})
	})
	if err == nil {
		logTaskStateChange(uc.logger, ctx, "action_item.complete_occurrence", userID, taskID, string(seriesID))
	}
	return err
}

func (uc *CompleteActionItemUseCase) Execute(ctx context.Context, userID domain.UserID, taskID domain.TaskID, actionItemID domain.ActionItemID) (err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "CompleteActionItemUseCase.Execute", err) }()

	asOf := time.Now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withTaskProgressMutationForPermission(ctx, repos, userID, taskID, asOf, shared.ActionItemUpdate(), func() error {
			return repos.ActionItems().CheckForOwnedTask(ctx, userID, taskID, actionItemID)
		})
	})
	if err == nil {
		logTaskStateChange(uc.logger, ctx, "action_item.complete", userID, taskID, string(actionItemID))
	}
	return err
}
