package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/todo/dao"
	"github.com/Najah7/task2todaytodo/internal/application/todo/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type RestoreScheduleUseCase struct {
	uow         UOW
	regenerator *RegenerateTodayUseCase
	logger      telemetry.Logger
}

func NewRestoreScheduleUseCase(uow UOW, newID func() string, loggers ...telemetry.Logger) *RestoreScheduleUseCase {
	var logger telemetry.Logger
	if len(loggers) > 0 {
		logger = loggers[0]
	}
	return &RestoreScheduleUseCase{uow: uow, regenerator: NewRegenerateTodayUseCase(uow, newID), logger: telemetry.OrNop(logger)}
}

func (u *RestoreScheduleUseCase) Execute(ctx context.Context, userID, itemID string, expectedRevision int32) (result dao.Todo, err error) {
	defer func() { logUnexpectedTodoFailure(u.logger, ctx, "RestoreScheduleUseCase.Execute", err) }()
	err = u.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		day, err := repos.Sources().CurrentDay(ctx, userID)
		if err != nil {
			return err
		}
		if err := repos.Todos().LockUser(ctx, userID); err != nil {
			return err
		}
		todo, err := repos.Todos().GetByUserDate(ctx, userID, day.Date)
		if err != nil {
			return err
		}
		if todo.Revision != expectedRevision {
			return ErrRevisionConflict
		}
		items, err := repos.Items().ListByTodoID(ctx, todo.ID)
		if err != nil {
			return err
		}
		var target *dao.Item
		for index := range items {
			if items[index].ID == itemID {
				target = &items[index]
				break
			}
		}
		if target == nil || target.Kind != string(domain.FrameSchedule) || !target.Hidden {
			return ErrFrameNotFound
		}
		schedules, err := repos.Sources().ReadSavedScheduleSources(ctx, userID, todo.Date, todo.Timezone)
		if err != nil {
			return err
		}
		occurrence, ok := findSchedule(schedules, scheduleKey(target.ScheduleRuleID, target.OccurrenceDate))
		if !ok || !occurrence.CanRestore {
			return ErrPermissionDenied
		}
		result, err = u.regenerator.RegenerateTodayInTx(ctx, userID, day.Date, repos)
		return err
	})
	if err == nil {
		logTodoStateChange(u.logger, ctx, "todo.schedule.restore", userID, result.ID, itemID)
	}
	return result, err
}
