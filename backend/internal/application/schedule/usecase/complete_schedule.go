package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type CompleteScheduleUseCase struct {
	uow    UOW
	ID     shared.ID
	logger logging.Logger
}

func NewCompleteScheduleUseCase(uow UOW, logger logging.Logger, ids ...shared.ID) *CompleteScheduleUseCase {
	var id shared.ID
	if len(ids) > 0 {
		id = ids[0]
	}
	return &CompleteScheduleUseCase{uow: uow, ID: id, logger: logging.OrNop(logger)}
}

func (uc *CompleteScheduleUseCase) ExecuteOccurrence(ctx context.Context, actorID domain.UserID, seriesID domain.ScheduleID, occurrenceDate string) (err error) {
	defer func() { logUnexpectedScheduleFailure(uc.logger, ctx, "CompleteScheduleUseCase.ExecuteOccurrence", err) }()
	asOf := time.Now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withScheduleProjectMutation(ctx, repos, actorID, seriesID, asOf, shared.ScheduleUpdate(), func(_ dao.Schedule, before scheduleProjectMutationSnapshots) error {
			state, err := loadOccurrence(ctx, repos.Schedules(), actorID, seriesID, occurrenceDate, asOf, shared.ScheduleUpdate())
			if err != nil {
				return err
			}
			if scheduleVirtualOccurrenceSuppressed(before, state, false) {
				return ErrOccurrenceInactive
			}
			if !scheduleIsRecurring(state.root) || state.rootOccurrence {
				id := domain.ScheduleID(state.root.ID)
				if state.current != nil {
					id = domain.ScheduleID(state.current.ID)
				}
				return repos.Schedules().SetCompletedByUserID(ctx, actorID, id, true, shared.ScheduleUpdate())
			}
			if state.current != nil {
				return repos.Schedules().SetCompletedByUserID(ctx, actorID, domain.ScheduleID(state.current.ID), true, shared.ScheduleUpdate())
			}
			id, err := generatedScheduleID(uc.ID)
			if err != nil {
				return err
			}
			schedule, err := scheduleDomainOccurrence(state.root, nil, state.date, id, true, asOf)
			if err != nil {
				return err
			}
			writer, err := requireOverride(repos.Schedules())
			if err != nil {
				return err
			}
			_, err = writer.UpsertOverrideByUserID(ctx, actorID, schedule)
			return err
		})
	})
	if err == nil {
		logScheduleStateChange(uc.logger, ctx, "schedule.complete_occurrence", actorID, seriesID)
	}
	return err
}

func (uc *CompleteScheduleUseCase) Execute(ctx context.Context, actorID domain.UserID, scheduleID domain.ScheduleID) (err error) {
	defer func() { logUnexpectedScheduleFailure(uc.logger, ctx, "CompleteScheduleUseCase.Execute", err) }()
	asOf := time.Now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withScheduleProjectMutation(ctx, repos, actorID, scheduleID, asOf, shared.ScheduleUpdate(), func(_ dao.Schedule, _ scheduleProjectMutationSnapshots) error {
			return repos.Schedules().SetCompletedByUserID(ctx, actorID, scheduleID, true, shared.ScheduleUpdate())
		})
	})
	if err == nil {
		logScheduleStateChange(uc.logger, ctx, "schedule.complete", actorID, scheduleID)
	}
	return err
}
