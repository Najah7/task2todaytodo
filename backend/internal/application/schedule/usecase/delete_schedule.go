package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type DeleteScheduleUseCase struct {
	uow    UOW
	logger logging.Logger
}

func NewDeleteScheduleUseCase(uow UOW, logger logging.Logger) *DeleteScheduleUseCase {
	return &DeleteScheduleUseCase{uow: uow, logger: logging.OrNop(logger)}
}

func (uc *DeleteScheduleUseCase) Execute(ctx context.Context, actorID domain.UserID, scheduleID domain.ScheduleID) (err error) {
	defer func() { logUnexpectedScheduleFailure(uc.logger, ctx, "DeleteScheduleUseCase.Execute", err) }()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		schedule, err := repos.Schedules().GetByUserIDWithPermission(ctx, actorID, scheduleID, shared.ScheduleDelete())
		if err != nil {
			return err
		}
		if schedule.RepeatState == repeatStateOneOff && schedule.Completed {
			return ErrOccurrenceCompleted
		}
		if schedule.Deleted {
			return ErrOccurrenceInactive
		}
		return repos.Schedules().TombstoneByUserID(ctx, actorID, scheduleID, shared.ScheduleDelete())
	})
	if err == nil {
		logScheduleStateChange(uc.logger, ctx, "schedule.delete", actorID, scheduleID)
	}
	return err
}

func (uc *DeleteScheduleUseCase) ExecuteOccurrence(ctx context.Context, actorID domain.UserID, scheduleID domain.ScheduleID, occurrenceDate string) (err error) {
	defer func() { logUnexpectedScheduleFailure(uc.logger, ctx, "DeleteScheduleUseCase.ExecuteOccurrence", err) }()
	asOf := time.Now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		state, err := loadOccurrence(ctx, repos.Schedules(), actorID, scheduleID, occurrenceDate, asOf, shared.ScheduleDelete())
		if err != nil {
			return err
		}
		if state.root.RepeatState == repeatStateOneOff && state.root.Completed {
			return ErrOccurrenceCompleted
		}
		if state.root.Deleted {
			return ErrOccurrenceInactive
		}
		id := domain.ScheduleID(state.root.ID)
		if state.current != nil {
			id = domain.ScheduleID(state.current.ID)
		}
		return repos.Schedules().TombstoneByUserID(ctx, actorID, id, shared.ScheduleDelete())
	})
	if err == nil {
		logScheduleStateChange(uc.logger, ctx, "schedule.delete_occurrence", actorID, scheduleID)
	}
	return err
}
