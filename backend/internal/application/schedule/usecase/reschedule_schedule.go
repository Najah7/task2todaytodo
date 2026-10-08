package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/shared/calendar"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type RescheduleScheduleInput struct {
	UserID         domain.UserID
	ScheduleID     domain.ScheduleID
	OccurrenceDate string
	StartAt        time.Time
	EndAt          time.Time
	Scope          string
}

type RescheduleScheduleUseCase struct {
	uow    UOW
	ID     shared.ID
	logger logging.Logger
}

func NewRescheduleScheduleUseCase(uow UOW, logger logging.Logger, ids ...shared.ID) *RescheduleScheduleUseCase {
	uc := &RescheduleScheduleUseCase{uow: uow, logger: logging.OrNop(logger)}
	if len(ids) > 0 {
		uc.ID = ids[0]
	}
	return uc
}

func (uc *RescheduleScheduleUseCase) Execute(ctx context.Context, input RescheduleScheduleInput) (dao.Schedule, error) {
	if input.OccurrenceDate != "" {
		return uc.ExecuteOccurrence(ctx, input)
	}
	if input.Scope != scopeCurrent && input.Scope != scopeFuture {
		return dao.Schedule{}, ErrScheduleScopeInvalid
	}
	var root dao.Schedule
	if err := uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		var err error
		root, err = repos.Schedules().GetByUserIDWithPermission(ctx, input.UserID, input.ScheduleID, shared.ScheduleUpdate())
		return err
	}); err != nil {
		logUnexpectedScheduleFailure(uc.logger, ctx, "schedule.reschedule.load", err)
		return dao.Schedule{}, err
	}
	if root.ID != string(input.ScheduleID) || root.SeriesID != root.ID {
		return dao.Schedule{}, domain.ErrScheduleNotFound
	}
	if root.IntervalWeeks != domain.OnceIntervalWeeks {
		return dao.Schedule{}, ErrOccurrenceDateRequired
	}
	input.OccurrenceDate = root.OccurrenceDate
	input.Scope = scopeCurrent
	return uc.ExecuteOccurrence(ctx, input)
}

func (uc *RescheduleScheduleUseCase) ExecuteOccurrence(ctx context.Context, input RescheduleScheduleInput) (output dao.Schedule, err error) {
	defer func() {
		logUnexpectedScheduleFailure(uc.logger, ctx, "RescheduleScheduleUseCase.ExecuteOccurrence", err)
	}()
	if input.Scope != scopeCurrent && input.Scope != scopeFuture {
		return dao.Schedule{}, ErrScheduleScopeInvalid
	}
	asOf := time.Now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withScheduleProjectMutation(ctx, repos, input.UserID, input.ScheduleID, asOf, shared.ScheduleUpdate(), func(_ dao.Schedule, before scheduleProjectMutationSnapshots) error {
			state, err := loadOccurrence(ctx, repos.Schedules(), input.UserID, input.ScheduleID, input.OccurrenceDate, asOf, shared.ScheduleUpdate())
			if err != nil {
				return err
			}
			if scheduleVirtualOccurrenceSuppressed(before, state, false) {
				return ErrOccurrenceInactive
			}
			var id domain.ScheduleID
			completed := false
			if state.current != nil {
				id, completed = domain.ScheduleID(state.current.ID), state.current.Completed
			} else if state.rootOccurrence {
				id = domain.ScheduleID(state.root.ID)
			} else if !scheduleIsRecurring(state.root) {
				id = domain.ScheduleID(state.root.ID)
			} else if input.Scope == scopeCurrent {
				id, err = generatedScheduleID(uc.ID)
				if err != nil {
					return err
				}
			} else {
				id = domain.ScheduleID(VirtualOccurrenceID)
			}
			schedule, err := scheduleDomainOccurrence(state.root, state.current, state.date, id, completed, asOf)
			if err != nil {
				return err
			}
			schedule.StartAt, schedule.EndAt = input.StartAt, input.EndAt
			validated, err := domain.NewScheduleWithRecurrence(schedule)
			if err != nil {
				return err
			}
			if !scheduleIsRecurring(state.root) || state.rootOccurrence {
				output, err = repos.Schedules().UpdateByUserID(ctx, input.UserID, validated, shared.ScheduleUpdate())
				return err
			}
			if input.Scope == scopeCurrent {
				writer, err := requireOverride(repos.Schedules())
				if err != nil {
					return err
				}
				savedID, err := writer.UpsertOverrideByUserID(ctx, input.UserID, validated)
				if err != nil {
					return err
				}
				output, err = repos.Schedules().GetByUserIDWithPermission(ctx, input.UserID, domain.ScheduleID(savedID), shared.ScheduleUpdate())
				return err
			}
			location, err := time.LoadLocation(state.root.Timezone)
			if err != nil {
				return err
			}
			if input.StartAt.In(location).Format("2006-01-02") != state.date.Format("2006-01-02") {
				return ErrRescheduleDateMismatch
			}
			rootSchedule, err := scheduleDomainOccurrence(state.root, nil, mustParseDate(state.root.OccurrenceDate), domain.ScheduleID(state.root.ID), state.root.Completed, asOf)
			if err != nil {
				return err
			}
			startLocal, endLocal := input.StartAt.In(location), input.EndAt.In(location)
			rootStart, valid := calendar.ResolveWallTime(mustParseDate(state.root.OccurrenceDate), startLocal, 0, location)
			if !valid {
				return ErrOccurrenceInactive
			}
			rootEnd := rootStart.Add(endLocal.Sub(startLocal))
			rootSchedule.StartAt, rootSchedule.EndAt = rootStart, rootEnd
			validated, err = domain.NewScheduleWithRecurrence(rootSchedule)
			if err != nil {
				return err
			}
			writer, err := requireSeriesTemplate(repos.Schedules())
			if err != nil {
				return err
			}
			snapshotID, err := generatedScheduleID(uc.ID)
			if err != nil {
				return err
			}
			output, err = writer.UpdateSeriesTemplateByUserID(ctx, input.UserID, input.ScheduleID, snapshotID, validated)
			return err
		})
	})
	if err == nil {
		logScheduleStateChange(uc.logger, ctx, "schedule.reschedule_occurrence", input.UserID, input.ScheduleID)
	}
	return output, err
}
