package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type SkipScheduleUseCase struct {
	uow    UOW
	logger logging.Logger
}

type RestoreScheduleUseCase struct {
	uow    UOW
	logger logging.Logger
}

func NewSkipScheduleUseCase(uow UOW, logger logging.Logger) *SkipScheduleUseCase {
	return &SkipScheduleUseCase{uow: uow, logger: logging.OrNop(logger)}
}

func NewRestoreScheduleUseCase(uow UOW, logger logging.Logger) *RestoreScheduleUseCase {
	return &RestoreScheduleUseCase{uow: uow, logger: logging.OrNop(logger)}
}

func (uc *SkipScheduleUseCase) Execute(ctx context.Context, actorID domain.UserID, seriesID domain.ScheduleID, occurrenceDate string) (err error) {
	defer func() { logUnexpectedScheduleFailure(uc.logger, ctx, "SkipScheduleUseCase.Execute", err) }()
	asOf := time.Now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withScheduleProjectMutation(ctx, repos, actorID, seriesID, asOf, shared.OccurrenceUpdate(), func(_ dao.Schedule, before scheduleProjectMutationSnapshots) error {
			state, err := loadOccurrence(ctx, repos.Schedules(), actorID, seriesID, occurrenceDate, asOf, shared.OccurrenceUpdate())
			if errors.Is(err, ErrOccurrenceInactive) && state.skipped {
				return nil
			}
			if err != nil {
				return err
			}
			if scheduleVirtualOccurrenceSuppressed(before, state, false) {
				return ErrOccurrenceInactive
			}
			if state.root.IntervalWeeks == domain.OnceIntervalWeeks {
				return ErrOccurrenceInactive
			}
			if state.current != nil && state.current.Completed {
				return ErrOccurrenceCompleted
			}
			store, err := requireSkippedOccurrenceStore(repos.Schedules())
			if err != nil {
				return err
			}
			return store.SetSkippedOccurrence(ctx, actorID, seriesID, state.date, true)
		})
	})
	if err == nil {
		logScheduleStateChange(uc.logger, ctx, "schedule.skip_occurrence", actorID, seriesID)
	}
	return err
}

func (uc *RestoreScheduleUseCase) Execute(ctx context.Context, actorID domain.UserID, seriesID domain.ScheduleID, occurrenceDate string) (err error) {
	defer func() { logUnexpectedScheduleFailure(uc.logger, ctx, "RestoreScheduleUseCase.Execute", err) }()
	asOf := time.Now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withScheduleProjectMutation(ctx, repos, actorID, seriesID, asOf, shared.OccurrenceUpdate(), func(_ dao.Schedule, before scheduleProjectMutationSnapshots) error {
			state, err := loadOccurrence(ctx, repos.Schedules(), actorID, seriesID, occurrenceDate, asOf, shared.OccurrenceUpdate())
			if errors.Is(err, ErrOccurrenceInactive) && state.skipped {
				err = nil
			}
			if err != nil {
				return err
			}
			if scheduleVirtualOccurrenceSuppressed(before, state, true) {
				return ErrOccurrenceInactive
			}
			if state.root.IntervalWeeks == domain.OnceIntervalWeeks {
				return ErrOccurrenceInactive
			}
			store, err := requireSkippedOccurrenceStore(repos.Schedules())
			if err != nil {
				return ErrOccurrenceNotFound
			}
			return store.SetSkippedOccurrence(ctx, actorID, seriesID, state.date, false)
		})
	})
	if err == nil {
		logScheduleStateChange(uc.logger, ctx, "schedule.restore_occurrence", actorID, seriesID)
	}
	return err
}
