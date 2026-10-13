package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/todo/dao"
	"github.com/Najah7/task2todaytodo/internal/application/todo/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type RestoreScheduleUseCase struct {
	uow    UOW
	logger telemetry.Logger
}

func NewRestoreScheduleUseCase(uow UOW, newID func() string, loggers ...telemetry.Logger) *RestoreScheduleUseCase {
	var logger telemetry.Logger
	if len(loggers) > 0 {
		logger = loggers[0]
	}
	return &RestoreScheduleUseCase{uow: uow, logger: telemetry.OrNop(logger)}
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
		model, err := restoredItem(*target)
		if err != nil {
			return err
		}
		model, err = model.SetHidden(false)
		if err != nil {
			return err
		}
		if _, err := repos.Items().Save(ctx, model); err != nil {
			return err
		}
		items, err = repos.Items().ListByTodoID(ctx, todo.ID)
		if err != nil {
			return err
		}
		models, err := scheduleRestoreOrder(items, itemID, occurrence.StartAt)
		if err != nil {
			return err
		}
		if err := repos.Items().SavePositions(ctx, todo.ID, models); err != nil {
			return err
		}
		return bumpTodoRevision(ctx, repos, &todo, &result)
	})
	if err == nil {
		logTodoStateChange(u.logger, ctx, "todo.schedule.restore", userID, result.ID, itemID)
	}
	return result, err
}

func scheduleRestoreOrder(items []dao.Item, restoredID string, start time.Time) ([]domain.TodoItem, error) {
	models := make([]domain.TodoItem, 0, len(items))
	found := false
	for _, item := range items {
		model, err := restoredItem(item)
		if err != nil {
			return nil, err
		}
		if item.ID == restoredID {
			found = true
			model, err = model.SetHidden(false)
			if err != nil {
				return nil, err
			}
			model.Start, model.End = &start, item.ScheduledEnd
		}
		models = append(models, model)
	}
	if !found {
		return nil, ErrFrameNotFound
	}
	return domain.InsertScheduleByStart(models, restoredID, start)
}
