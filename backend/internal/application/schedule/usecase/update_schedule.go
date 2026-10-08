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

var (
	ErrSchedulePatchRequiredFieldNull = errors.New("required schedule field cannot be null")
)

type UpdateScheduleUseCase struct {
	uow    UOW
	ID     shared.ID
	now    func() time.Time
	logger logging.Logger
}

func NewUpdateScheduleUseCase(uow UOW, logger logging.Logger, ids ...shared.ID) *UpdateScheduleUseCase {
	uc := &UpdateScheduleUseCase{uow: uow, now: time.Now, logger: logging.OrNop(logger)}
	if len(ids) > 0 {
		uc.ID = ids[0]
	}
	return uc
}

func (uc *UpdateScheduleUseCase) ExecuteOccurrence(ctx context.Context, actorID domain.UserID, seriesID domain.ScheduleID, occurrenceDate, scope string, title PatchField[string], description PatchField[string], location PatchField[string]) (output dao.Schedule, err error) {
	defer func() { logUnexpectedScheduleFailure(uc.logger, ctx, "UpdateScheduleUseCase.ExecuteOccurrence", err) }()
	if scope != scopeCurrent && scope != scopeFuture {
		return dao.Schedule{}, ErrScheduleScopeInvalid
	}
	if title.Present && title.Value == nil {
		return dao.Schedule{}, ErrSchedulePatchRequiredFieldNull
	}
	asOf := uc.now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withScheduleProjectMutation(ctx, repos, actorID, seriesID, asOf, shared.ScheduleUpdate(), func(_ dao.Schedule, before scheduleProjectMutationSnapshots) error {
			state, err := loadOccurrence(ctx, repos.Schedules(), actorID, seriesID, occurrenceDate, asOf, shared.ScheduleUpdate())
			if err != nil {
				return err
			}
			if scheduleVirtualOccurrenceSuppressed(before, state, false) {
				return ErrOccurrenceInactive
			}
			if scope == scopeFuture {
				occurrenceID := domain.ScheduleID(VirtualOccurrenceID)
				if state.date.Equal(mustParseDate(state.root.OccurrenceDate)) {
					occurrenceID = domain.ScheduleID(state.root.ID)
				}
				schedule, err := scheduleDomainOccurrence(state.root, nil, state.date, occurrenceID, false, asOf)
				if err != nil {
					return err
				}
				applySchedulePatch(&schedule, title, description, location)
				rootSchedule, err := scheduleDomainOccurrence(state.root, nil, mustParseDate(state.root.OccurrenceDate), domain.ScheduleID(state.root.ID), state.root.Completed, asOf)
				if err != nil {
					return err
				}
				rootSchedule.Title, rootSchedule.Description, rootSchedule.Location = schedule.Title, schedule.Description, schedule.Location
				validated, err := domain.NewScheduleWithRecurrence(rootSchedule)
				if err != nil {
					return err
				}
				if !scheduleIsRecurring(state.root) {
					output, err = repos.Schedules().UpdateByUserID(ctx, actorID, validated, shared.ScheduleUpdate())
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
				output, err = writer.UpdateSeriesTemplateByUserID(ctx, actorID, seriesID, snapshotID, validated)
				return err
			}
			var id domain.ScheduleID
			completed := false
			if state.current != nil {
				id, completed = domain.ScheduleID(state.current.ID), state.current.Completed
			} else if state.rootOccurrence {
				id = domain.ScheduleID(state.root.ID)
			} else if !scheduleIsRecurring(state.root) {
				id = domain.ScheduleID(state.root.ID)
			} else {
				id, err = generatedScheduleID(uc.ID)
				if err != nil {
					return err
				}
			}
			schedule, err := scheduleDomainOccurrence(state.root, state.current, state.date, id, completed, asOf)
			if err != nil {
				return err
			}
			applySchedulePatch(&schedule, title, description, location)
			validated, err := domain.NewScheduleWithRecurrence(schedule)
			if err != nil {
				return err
			}
			if !scheduleIsRecurring(state.root) || state.rootOccurrence {
				output, err = repos.Schedules().UpdateByUserID(ctx, actorID, validated, shared.ScheduleUpdate())
				return err
			}
			writer, err := requireOverride(repos.Schedules())
			if err != nil {
				return err
			}
			savedID, err := writer.UpsertOverrideByUserID(ctx, actorID, validated)
			if err != nil {
				return err
			}
			output, err = repos.Schedules().GetByUserIDWithPermission(ctx, actorID, domain.ScheduleID(savedID), shared.ScheduleUpdate())
			return err
		})
	})
	if err == nil {
		logScheduleStateChange(uc.logger, ctx, "schedule.update_occurrence", actorID, seriesID)
	}
	return output, err
}

func (uc *UpdateScheduleUseCase) Execute(ctx context.Context, actorID domain.UserID, scheduleID domain.ScheduleID, scope string, title PatchField[string], description PatchField[string], location PatchField[string]) (dao.Schedule, error) {
	var row dao.Schedule
	if err := uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		var err error
		row, err = repos.Schedules().GetByUserIDWithPermission(ctx, actorID, scheduleID, shared.ScheduleUpdate())
		return err
	}); err != nil {
		logUnexpectedScheduleFailure(uc.logger, ctx, "schedule.update.load", err)
		return dao.Schedule{}, err
	}
	if row.ID != string(scheduleID) {
		return dao.Schedule{}, domain.ErrScheduleNotFound
	}
	return uc.ExecuteOccurrence(ctx, actorID, domain.ScheduleID(row.SeriesID), row.OccurrenceDate, scope, title, description, location)
}

func applySchedulePatch(schedule *domain.Schedule, title, description, location PatchField[string]) {
	if title.Present {
		schedule.Title = *title.Value
	}
	if description.Present {
		schedule.Description = ""
		if description.Value != nil {
			schedule.Description = *description.Value
		}
	}
	if location.Present {
		schedule.Location = ""
		if location.Value != nil {
			schedule.Location = *location.Value
		}
	}
}
