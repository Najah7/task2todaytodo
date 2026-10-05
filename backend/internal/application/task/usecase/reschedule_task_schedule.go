package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	tasktime "github.com/Najah7/task2todaytodo/internal/application/task"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

var ErrRescheduleDateMismatch = errors.New("future reschedule must keep the occurrence's local date")

type RescheduleTaskScheduleInput struct {
	UserID         domain.UserID
	TaskID         domain.TaskID
	TaskScheduleID domain.TaskScheduleID
	OccurrenceDate string
	StartAt        time.Time
	EndAt          time.Time
	Scope          string
}

type RescheduleTaskScheduleUseCase struct {
	uow    UOW
	ID     shared.ID
	logger logging.Logger
}

func NewRescheduleTaskScheduleUseCase(uow UOW, logger logging.Logger, ids ...shared.ID) *RescheduleTaskScheduleUseCase {
	uc := &RescheduleTaskScheduleUseCase{uow: uow, logger: logging.OrNop(logger)}
	if len(ids) > 0 {
		uc.ID = ids[0]
	}
	return uc
}

func (uc *RescheduleTaskScheduleUseCase) Execute(ctx context.Context, input RescheduleTaskScheduleInput) (dao.TaskSchedule, error) {
	if input.OccurrenceDate != "" {
		return uc.ExecuteOccurrence(ctx, input)
	}
	if input.Scope != taskScheduleScopeCurrent && input.Scope != taskScheduleScopeFuture {
		return dao.TaskSchedule{}, ErrTaskScheduleScopeInvalid
	}

	var root dao.TaskSchedule
	if err := uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		var err error
		root, err = getTaskScheduleForCommand(ctx, repos.TaskSchedules(), input.UserID, input.TaskID, input.TaskScheduleID, shared.TaskScheduleUpdate())
		return err
	}); err != nil {
		logUnexpectedTaskFailure(uc.logger, ctx, "task_schedule.reschedule.load", err)
		return dao.TaskSchedule{}, err
	}
	if root.ID != string(input.TaskScheduleID) || root.TaskID != string(input.TaskID) || root.SeriesID != root.ID {
		return dao.TaskSchedule{}, domain.ErrTaskScheduleNotFound
	}
	if root.IntervalWeeks != domain.OnceIntervalWeeks {
		return dao.TaskSchedule{}, ErrOccurrenceDateRequired
	}
	input.OccurrenceDate = root.OccurrenceDate
	input.Scope = taskScheduleScopeCurrent
	return uc.ExecuteOccurrence(ctx, input)
}

func (uc *RescheduleTaskScheduleUseCase) ExecuteOccurrence(ctx context.Context, input RescheduleTaskScheduleInput) (output dao.TaskSchedule, err error) {
	defer func() {
		logUnexpectedTaskFailure(uc.logger, ctx, "RescheduleTaskScheduleUseCase.ExecuteOccurrence", err)
	}()
	if input.Scope != taskScheduleScopeCurrent && input.Scope != taskScheduleScopeFuture {
		return dao.TaskSchedule{}, ErrTaskScheduleScopeInvalid
	}
	asOf := time.Now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		mutate := func() error {
			state, err := loadTaskScheduleOccurrenceWithCapability(ctx, repos, input.UserID, input.TaskID, input.TaskScheduleID, input.OccurrenceDate, asOf, shared.TaskScheduleUpdate())
			if err != nil {
				return err
			}
			var id domain.TaskScheduleID
			completed := false
			if state.current != nil {
				id = domain.TaskScheduleID(state.current.ID)
				completed = state.current.Completed
			} else if !taskScheduleIsRecurring(state.root) {
				id = domain.TaskScheduleID(state.root.ID)
			} else if input.Scope == taskScheduleScopeCurrent {
				id, err = generatedTaskScheduleID(uc.ID)
				if err != nil {
					return err
				}
			} else {
				id = domain.TaskScheduleID(VirtualOccurrenceID)
			}
			schedule, err := taskScheduleDomainOccurrence(state.root, state.current, state.date, id, completed, asOf)
			if err != nil {
				return err
			}
			schedule.StartAt, schedule.EndAt = input.StartAt, input.EndAt
			validated, err := domain.NewTaskScheduleWithRecurrence(schedule)
			if err != nil {
				return err
			}
			if !taskScheduleIsRecurring(state.root) {
				output, err = repos.TaskSchedules().UpdateByTaskAndUserID(ctx, input.UserID, validated)
				return err
			}
			if input.Scope == taskScheduleScopeCurrent {
				writer, err := requireScheduleOverride(repos.TaskSchedules())
				if err != nil {
					return err
				}
				savedID, err := writer.UpsertTaskScheduleOverride(ctx, input.UserID, validated)
				if err != nil {
					return err
				}
				output = scheduleDAOFromOccurrence(state.root, validated, savedID)
				return nil
			}
			location, err := time.LoadLocation(state.root.Timezone)
			if err != nil {
				return domain.ErrRecurrenceTimezoneInvalid
			}
			if state.root.IntervalWeeks > domain.OnceIntervalWeeks && input.StartAt.In(location).Format("2006-01-02") != state.date.Format("2006-01-02") {
				return ErrRescheduleDateMismatch
			}
			rootSchedule, err := taskScheduleDomainOccurrence(state.root, nil, mustParseDate(state.root.OccurrenceDate), domain.TaskScheduleID(state.root.ID), state.root.Completed, asOf)
			if err != nil {
				return err
			}
			start, end := input.StartAt.In(location), input.EndAt.In(location)
			rootStart, valid := tasktime.ResolveWallTime(mustParseDate(state.root.OccurrenceDate), start, 0, location)
			if !valid {
				return ErrOccurrenceInactive
			}
			rootEnd := rootStart.Add(end.Sub(start))
			rootSchedule.StartAt, rootSchedule.EndAt = rootStart, rootEnd
			validated, err = domain.NewTaskScheduleWithRecurrence(rootSchedule)
			if err != nil {
				return err
			}
			writer, ok := repos.TaskSchedules().(scheduleSeriesTemplateWriter)
			if !ok {
				return ErrOccurrenceInactive
			}
			snapshotID, err := generatedTaskScheduleID(uc.ID)
			if err != nil {
				return err
			}
			output, err = writer.UpdateTaskScheduleSeriesTemplate(ctx, input.UserID, input.TaskID, input.TaskScheduleID, snapshotID, validated)
			return err
		}
		return withTaskProgressMutationForPermission(ctx, repos, input.UserID, input.TaskID, asOf, shared.TaskScheduleUpdate(), mutate)
	})
	if err == nil {
		logTaskStateChange(uc.logger, ctx, "task_schedule.reschedule_occurrence", input.UserID, input.TaskID, string(input.TaskScheduleID))
	}
	return output, err
}
