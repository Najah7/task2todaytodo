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

var (
	ErrTaskSchedulePatchRequiredFieldNull = errors.New("required task schedule field cannot be null")
	ErrTaskScheduleScopeInvalid           = errors.New("task schedule scope must be current or future")
)

const (
	taskScheduleScopeCurrent = "current"
	taskScheduleScopeFuture  = "future"
)

type UpdateTaskScheduleUseCase struct {
	uow    UOW
	ID     shared.ID
	now    func() time.Time
	logger logging.Logger
}

func NewUpdateTaskScheduleUseCase(uow UOW, logger logging.Logger, ids ...shared.ID) *UpdateTaskScheduleUseCase {
	uc := &UpdateTaskScheduleUseCase{uow: uow, now: time.Now, logger: logging.OrNop(logger)}
	if len(ids) > 0 {
		uc.ID = ids[0]
	}
	return uc
}

func (uc *UpdateTaskScheduleUseCase) ExecuteOccurrence(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.TaskScheduleID, occurrenceDate, scope string, title PatchField[string], description PatchField[string], location PatchField[string]) (output dao.TaskSchedule, err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "UpdateTaskScheduleUseCase.ExecuteOccurrence", err) }()

	asOf := uc.now()
	if scope == taskScheduleScopeFuture {
		if title.Present && title.Value == nil {
			return dao.TaskSchedule{}, ErrTaskSchedulePatchRequiredFieldNull
		}
		var result dao.TaskSchedule
		err := uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
			return withTaskProgressMutationForPermission(ctx, repos, userID, taskID, asOf, shared.TaskScheduleUpdate(), func() error {
				state, err := loadTaskScheduleOccurrenceWithCapability(ctx, repos, userID, taskID, seriesID, occurrenceDate, asOf, shared.TaskScheduleUpdate())
				if err != nil {
					return err
				}
				occurrenceID := domain.TaskScheduleID(VirtualOccurrenceID)
				if state.date.Equal(mustParseDate(state.root.OccurrenceDate)) {
					occurrenceID = domain.TaskScheduleID(state.root.ID)
				}
				schedule, err := taskScheduleDomainOccurrence(state.root, nil, state.date, occurrenceID, false, asOf)
				if err != nil {
					return err
				}
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
				item, err := taskScheduleDomainOccurrence(state.root, nil, mustParseDate(state.root.OccurrenceDate), domain.TaskScheduleID(state.root.ID), state.root.Completed, asOf)
				if err != nil {
					return err
				}
				item.Title, item.Description, item.Location = schedule.Title, schedule.Description, schedule.Location
				validated, err := domain.NewTaskScheduleWithRecurrence(item)
				if err != nil {
					return err
				}
				if !taskScheduleIsRecurring(state.root) {
					result, err = repos.TaskSchedules().UpdateByTaskAndUserID(ctx, userID, validated)
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
				result, err = writer.UpdateTaskScheduleSeriesTemplate(ctx, userID, taskID, seriesID, snapshotID, validated)
				return err
			})
		})
		return result, err
	}
	if scope != taskScheduleScopeCurrent {
		return dao.TaskSchedule{}, ErrTaskScheduleScopeInvalid
	}
	if title.Present && title.Value == nil {
		return dao.TaskSchedule{}, ErrTaskSchedulePatchRequiredFieldNull
	}
	var result dao.TaskSchedule
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withTaskProgressMutationForPermission(ctx, repos, userID, taskID, asOf, shared.TaskScheduleUpdate(), func() error {
			state, err := loadTaskScheduleOccurrenceWithCapability(ctx, repos, userID, taskID, seriesID, occurrenceDate, asOf, shared.TaskScheduleUpdate())
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
			} else {
				id, err = generatedTaskScheduleID(uc.ID)
				if err != nil {
					return err
				}
			}
			schedule, err := taskScheduleDomainOccurrence(state.root, state.current, state.date, id, completed, asOf)
			if err != nil {
				return err
			}
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
			validated, err := domain.NewTaskScheduleWithRecurrence(schedule)
			if err != nil {
				return err
			}
			if !taskScheduleIsRecurring(state.root) {
				result, err = repos.TaskSchedules().UpdateByTaskAndUserID(ctx, userID, validated)
				return err
			}
			writer, err := requireScheduleOverride(repos.TaskSchedules())
			if err != nil {
				return err
			}
			savedID, err := writer.UpsertTaskScheduleOverride(ctx, userID, validated)
			if err != nil {
				return err
			}
			result = scheduleDAOFromOccurrence(state.root, validated, savedID)
			return nil
		})
	})
	return result, err

}

func (uc *UpdateTaskScheduleUseCase) Execute(
	ctx context.Context,
	userID domain.UserID,
	taskID domain.TaskID,
	scheduleID domain.TaskScheduleID,
	scope string,
	title PatchField[string],
	description PatchField[string],
	location PatchField[string],
) (dao.TaskSchedule, error) {
	var row dao.TaskSchedule
	if err := uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		var err error
		row, err = getTaskScheduleForCommand(ctx, repos.TaskSchedules(), userID, taskID, scheduleID, shared.TaskScheduleUpdate())
		return err
	}); err != nil {
		logUnexpectedTaskFailure(uc.logger, ctx, "task_schedule.update.load", err)
		return dao.TaskSchedule{}, err
	}
	if row.ID != string(scheduleID) || row.TaskID != string(taskID) {
		return dao.TaskSchedule{}, domain.ErrTaskScheduleNotFound
	}
	return uc.ExecuteOccurrence(ctx, userID, taskID, domain.TaskScheduleID(row.SeriesID), row.OccurrenceDate, scope, title, description, location)
}
