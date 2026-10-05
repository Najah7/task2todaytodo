package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type reopenTaskScheduleUOW interface {
	Do(ctx context.Context, fn func(context.Context, Repositories) error) error
}

type ReopenTaskScheduleUseCase struct {
	uow    reopenTaskScheduleUOW
	logger logging.Logger
}

func NewReopenTaskScheduleUseCase(uow reopenTaskScheduleUOW, logger logging.Logger) *ReopenTaskScheduleUseCase {
	return &ReopenTaskScheduleUseCase{logger: logging.OrNop(logger), uow: uow}
}

func (uc *ReopenTaskScheduleUseCase) Execute(ctx context.Context, userID domain.UserID, taskID domain.TaskID, scheduleID domain.TaskScheduleID) (err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "ReopenTaskScheduleUseCase.Execute", err) }()

	asOf := time.Now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withTaskProgressMutationForPermission(ctx, repos, userID, taskID, asOf, shared.TaskScheduleUpdate(), func() error {
			return repos.TaskSchedules().SetCompletedForOwnedTask(ctx, userID, taskID, scheduleID, false)
		})
	})
	if err == nil {
		logTaskStateChange(uc.logger, ctx, "task_schedule.reopen", userID, taskID, string(scheduleID))
	}
	return err
}

func (uc *ReopenTaskScheduleUseCase) ExecuteOccurrence(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.TaskScheduleID, occurrenceDate string) (err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "ReopenTaskScheduleUseCase.ExecuteOccurrence", err) }()

	asOf := time.Now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withTaskProgressMutationForPermission(ctx, repos, userID, taskID, asOf, shared.TaskScheduleUpdate(), func() error {
			state, err := loadTaskScheduleOccurrenceWithCapability(ctx, repos, userID, taskID, seriesID, occurrenceDate, asOf, shared.TaskScheduleUpdate())
			if err != nil {
				return err
			}
			if !taskScheduleIsRecurring(state.root) {
				return repos.TaskSchedules().SetCompletedForOwnedTask(ctx, userID, taskID, domain.TaskScheduleID(state.root.ID), false)
			}
			if state.current == nil || !state.current.Completed {
				return nil
			}
			return repos.TaskSchedules().SetCompletedForOwnedTask(ctx, userID, taskID, domain.TaskScheduleID(state.current.ID), false)
		})
	})
	if err == nil {
		logTaskStateChange(uc.logger, ctx, "task_schedule.reopen_occurrence", userID, taskID, string(seriesID))
	}
	return err
}
