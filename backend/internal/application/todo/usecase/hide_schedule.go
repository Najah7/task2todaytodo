package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/todo/dao"
	"github.com/Najah7/task2todaytodo/internal/application/todo/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type HideScheduleUseCase struct {
	uow    UOW
	newID  func() string
	logger telemetry.Logger
}

func NewHideScheduleUseCase(uow UOW, newID func() string, loggers ...telemetry.Logger) *HideScheduleUseCase {
	var logger telemetry.Logger
	if len(loggers) > 0 {
		logger = loggers[0]
	}
	return &HideScheduleUseCase{uow: uow, newID: newID, logger: telemetry.OrNop(logger)}
}

func (u *HideScheduleUseCase) Execute(ctx context.Context, userID, itemID string, expectedRevision int32) (result dao.Todo, err error) {
	defer func() { logUnexpectedTodoFailure(u.logger, ctx, "HideScheduleUseCase.Execute", err) }()
	err = u.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		day, err := repos.Sources().CurrentDay(ctx, userID)
		if err != nil {
			return err
		}
		if err := repos.Todos().LockUser(ctx, userID); err != nil {
			return err
		}
		todo, saved, sources, in, err := loadPlanningState(ctx, userID, day.Date, repos)
		if err != nil {
			return err
		}
		if todo.Revision != expectedRevision {
			return ErrRevisionConflict
		}
		var target *dao.Item
		for index := range saved {
			if saved[index].ID == itemID {
				target = &saved[index]
				break
			}
		}
		if target == nil || target.Kind != string(domain.FrameSchedule) {
			return ErrFrameNotFound
		}
		occurrence, ok := findSchedule(sources.Schedules, scheduleKey(target.ScheduleRuleID, target.OccurrenceDate))
		if !ok || !occurrence.CanHide {
			return ErrPermissionDenied
		}
		if target.Completed || occurrence.Completed {
			return ErrScheduleCompleted
		}
		model, err := restoredItem(*target)
		if err != nil {
			return err
		}
		hidden, err := model.SetHidden(true)
		if err != nil {
			return err
		}
		if _, err := repos.Items().Save(ctx, hidden); err != nil {
			return err
		}
		for index := range in.Schedules {
			if in.Schedules[index].ID == scheduleKey(target.ScheduleRuleID, target.OccurrenceDate) {
				in.Schedules[index].Hidden = true
			}
		}
		var order []domain.OrderedItem
		for _, item := range saved {
			if item.ID == itemID {
				continue
			}
			order = append(order, orderedItemsForSaved([]dao.Item{item})[0])
		}
		plan, err := domain.Reflow(domain.ReflowInput{Input: in, Order: order})
		if err != nil {
			return err
		}
		saved[targetIndex(saved, itemID)].Hidden = true
		if err := persistPlan(ctx, todo.ID, saved, plan, u.newID, repos); err != nil {
			return err
		}
		return bumpTodoRevision(ctx, repos, &todo, &result)
	})
	if err == nil {
		logTodoStateChange(u.logger, ctx, "todo.schedule.hide", userID, result.ID, itemID)
	}
	return result, err
}

func targetIndex(saved []dao.Item, id string) int {
	for index, item := range saved {
		if item.ID == id {
			return index
		}
	}
	return -1
}
