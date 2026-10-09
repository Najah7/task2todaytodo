package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type SkipActionItemUseCase struct {
	uow    UOW
	logger logging.Logger
}
type RestoreActionItemUseCase struct {
	uow    UOW
	logger logging.Logger
}

func NewSkipActionItemUseCase(uow UOW, logger logging.Logger) *SkipActionItemUseCase {
	return &SkipActionItemUseCase{logger: logging.OrNop(logger), uow: uow}
}
func NewRestoreActionItemUseCase(uow UOW, logger logging.Logger) *RestoreActionItemUseCase {
	return &RestoreActionItemUseCase{logger: logging.OrNop(logger), uow: uow}
}
func (uc *SkipActionItemUseCase) Execute(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.ActionItemID, occurrenceDate string) (err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "SkipActionItemUseCase.Execute", err) }()

	asOf := time.Now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withTaskProgressMutationForPermissionAndState(ctx, repos, userID, taskID, asOf, shared.OccurrenceUpdate(), func(_ dao.Task, projectBefore taskProjectMutationSnapshot) error {
			state, err := loadActionItemOccurrenceWithCapability(ctx, repos, userID, taskID, seriesID, occurrenceDate, asOf, shared.OccurrenceUpdate())
			if errors.Is(err, ErrOccurrenceInactive) && state.skipped {
				return nil
			}
			if err != nil {
				return err
			}
			if state.root.IntervalWeeks == domain.OnceIntervalWeeks {
				return ErrOccurrenceInactive
			}
			if state.current != nil && state.current.Completed {
				return ErrOccurrenceCompleted
			}
			if projectBefore.State.Status == "done" && !state.saved {
				return ErrOccurrenceInactive
			}
			store, ok := repos.ActionItems().(actionItemSkippedOccurrenceStore)
			if !ok {
				return ErrOccurrenceInactive
			}
			return store.SetActionItemSkippedOccurrence(ctx, userID, taskID, seriesID, skippedDate(state.date), true)
		})
	})
	if err == nil {
		logTaskStateChange(uc.logger, ctx, "action_item.skip_occurrence", userID, taskID, string(seriesID))
	}
	return err
}

func (uc *RestoreActionItemUseCase) Execute(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.ActionItemID, occurrenceDate string) (err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "RestoreActionItemUseCase.Execute", err) }()

	asOf := time.Now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withTaskProgressMutationForPermission(ctx, repos, userID, taskID, asOf, shared.OccurrenceUpdate(), func() error {
			state, err := loadActionItemOccurrenceWithCapability(ctx, repos, userID, taskID, seriesID, occurrenceDate, asOf, shared.OccurrenceUpdate())
			if errors.Is(err, ErrOccurrenceInactive) && state.skipped {
				err = nil
			}
			if err != nil {
				return err
			}
			if state.root.IntervalWeeks == domain.OnceIntervalWeeks {
				return ErrOccurrenceInactive
			}
			store, ok := repos.ActionItems().(actionItemSkippedOccurrenceStore)
			if !ok {
				return ErrOccurrenceNotFound
			}
			return store.SetActionItemSkippedOccurrence(ctx, userID, taskID, seriesID, skippedDate(state.date), false)
		})
	})
	if err == nil {
		logTaskStateChange(uc.logger, ctx, "action_item.restore_occurrence", userID, taskID, string(seriesID))
	}
	return err
}
