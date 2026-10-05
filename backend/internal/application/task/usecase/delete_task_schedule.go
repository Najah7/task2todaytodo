package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type DeleteTaskScheduleUseCase struct {
	uow    UOW
	logger logging.Logger
}

func NewDeleteTaskScheduleUseCase(uow UOW, logger logging.Logger) *DeleteTaskScheduleUseCase {
	return &DeleteTaskScheduleUseCase{logger: logging.OrNop(logger), uow: uow}
}

func (uc *DeleteTaskScheduleUseCase) Execute(ctx context.Context, userID domain.UserID, taskID domain.TaskID, taskScheduleID domain.TaskScheduleID) (err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "DeleteTaskScheduleUseCase.Execute", err) }()

	asOf := time.Now()
	return uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withTaskProgressMutationForPermission(ctx, repos, userID, taskID, asOf, shared.TaskScheduleDelete(), func() error {
			schedules := repos.TaskSchedules()
			schedule, err := getTaskScheduleForCommand(ctx, schedules, userID, taskID, taskScheduleID, shared.TaskScheduleDelete())
			if err != nil {
				return err
			}

			if schedule.RepeatState == repeatStateOneOff && schedule.Completed {
				return ErrOccurrenceCompleted
			}
			if schedule.Deleted {
				return ErrOccurrenceInactive
			}
			if schedule.ID == schedule.SeriesID {
				// Deleting the root suppresses future projections while retaining its
				// repeat metadata for saved occurrence history.
				if err := schedules.TombstoneByTaskAndUserID(ctx, userID, taskID, taskScheduleID); err != nil {
					return err
				}
				return nil
			}

			// Keep the deleted occurrence row so this date stays suppressed.
			return schedules.TombstoneByTaskAndUserID(ctx, userID, taskID, taskScheduleID)
		})
	})

}

func (uc *DeleteTaskScheduleUseCase) DeleteSeries(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.TaskScheduleID) (err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "DeleteTaskScheduleUseCase.DeleteSeries", err) }()

	return uc.Execute(ctx, userID, taskID, seriesID)

}
