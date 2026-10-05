package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type completeTaskScheduleUOW interface {
	Do(ctx context.Context, fn func(context.Context, Repositories) error) error
}

type CompleteTaskScheduleUseCase struct {
	uow    completeTaskScheduleUOW
	ID     shared.ID
	logger logging.Logger
}

func NewCompleteTaskScheduleUseCase(uow completeTaskScheduleUOW, logger logging.Logger, ids ...shared.ID) *CompleteTaskScheduleUseCase {
	var ID shared.ID
	if len(ids) > 0 {
		ID = ids[0]
	}
	return &CompleteTaskScheduleUseCase{logger: logging.OrNop(logger), uow: uow, ID: ID}
}

func (uc *CompleteTaskScheduleUseCase) ExecuteOccurrence(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.TaskScheduleID, occurrenceDate string) (err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "CompleteTaskScheduleUseCase.ExecuteOccurrence", err) }()

	asOf := time.Now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withTaskProgressMutationForPermission(ctx, repos, userID, taskID, asOf, shared.TaskScheduleUpdate(), func() error {
			state, err := loadTaskScheduleOccurrenceWithCapability(ctx, repos, userID, taskID, seriesID, occurrenceDate, asOf, shared.TaskScheduleUpdate())
			if err != nil {
				return err
			}
			if !taskScheduleIsRecurring(state.root) {
				return repos.TaskSchedules().SetCompletedForOwnedTask(ctx, userID, taskID, domain.TaskScheduleID(state.root.ID), true)
			}
			if state.current != nil {
				return repos.TaskSchedules().SetCompletedForOwnedTask(ctx, userID, taskID, domain.TaskScheduleID(state.current.ID), true)
			}
			id, err := generatedTaskScheduleID(uc.ID)
			if err != nil {
				return err
			}
			schedule, err := taskScheduleDomainOccurrence(state.root, nil, state.date, id, true, asOf)
			if err != nil {
				return err
			}
			writer, err := requireScheduleOverride(repos.TaskSchedules())
			if err != nil {
				return err
			}
			_, err = writer.UpsertTaskScheduleOverride(ctx, userID, schedule)
			return err
		})
	})
	if err == nil {
		logTaskStateChange(uc.logger, ctx, "task_schedule.complete_occurrence", userID, taskID, string(seriesID))
	}
	return err
}

func (uc *CompleteTaskScheduleUseCase) Execute(ctx context.Context, userID domain.UserID, taskID domain.TaskID, scheduleID domain.TaskScheduleID) (err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "CompleteTaskScheduleUseCase.Execute", err) }()

	asOf := time.Now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withTaskProgressMutationForPermission(ctx, repos, userID, taskID, asOf, shared.TaskScheduleUpdate(), func() error {
			return repos.TaskSchedules().SetCompletedForOwnedTask(ctx, userID, taskID, scheduleID, true)
		})
	})
	if err == nil {
		logTaskStateChange(uc.logger, ctx, "task_schedule.complete", userID, taskID, string(scheduleID))
	}
	return err
}
