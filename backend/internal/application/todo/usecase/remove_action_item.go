package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/todo/dao"
	"github.com/Najah7/task2todaytodo/internal/application/todo/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type RemoveActionItemUseCase struct {
	uow    UOW
	newID  func() string
	logger telemetry.Logger
}

func NewRemoveActionItemUseCase(uow UOW, newID func() string, loggers ...telemetry.Logger) *RemoveActionItemUseCase {
	var logger telemetry.Logger
	if len(loggers) > 0 {
		logger = loggers[0]
	}
	return &RemoveActionItemUseCase{uow: uow, newID: newID, logger: telemetry.OrNop(logger)}
}

func (u *RemoveActionItemUseCase) Execute(ctx context.Context, userID, frameID string, expectedRevision int32) (result dao.Todo, err error) {
	defer func() { logUnexpectedTodoFailure(u.logger, ctx, "RemoveActionItemUseCase.Execute", err) }()
	var workRef dao.WorkSourceRef
	err = u.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		day, err := repos.Sources().CurrentDay(ctx, userID)
		if err != nil {
			return err
		}
		if err := repos.Todos().LockUser(ctx, userID); err != nil {
			return err
		}
		todo, saved, _, in, err := loadPlanningState(ctx, userID, day.Date, repos)
		if err != nil {
			return err
		}
		if todo.Revision != expectedRevision {
			return ErrRevisionConflict
		}
		var requested *dao.Item
		for index := range saved {
			if saved[index].ID == frameID && domain.FrameKind(saved[index].Kind).IsWork() {
				requested = &saved[index]
				break
			}
		}
		if requested == nil {
			return ErrFrameNotFound
		}
		workRef = itemRef(*requested)
		workSources, err := repos.Sources().ReadWorkSources(ctx, userID, todo.Date, todo.Timezone, []dao.WorkSourceRef{workRef})
		if err != nil {
			return err
		}
		source, ok := sourceByRef(workSources, workRef)
		if !ok {
			if workRef.Kind == string(domain.FrameTask) {
				return ErrTaskNotFound
			}
			return ErrActionItemNotFound
		}
		if !source.CanRemoveFromToday {
			return ErrPermissionDenied
		}
		if source.HasCompletedFrame || source.Completed {
			if workRef.Kind == string(domain.FrameTask) {
				return ErrTaskStarted
			}
			return ErrActionItemStarted
		}
		found := false
		for _, item := range saved {
			if item.Kind == workRef.Kind && itemSourceID(item) == workRef.ID {
				found = true
				if item.Completed {
					if workRef.Kind == string(domain.FrameTask) {
						return ErrTaskStarted
					}
					return ErrActionItemStarted
				}
			}
		}
		if !found {
			return ErrFrameNotFound
		}
		order := make([]domain.OrderedItem, 0, len(saved))
		for _, item := range saved {
			if item.Kind == workRef.Kind && itemSourceID(item) == workRef.ID && !item.Completed {
				continue
			}
			order = append(order, orderedItemsForSaved([]dao.Item{item})[0])
		}
		plan, err := domain.Reflow(domain.ReflowInput{Input: in, Order: order})
		if err != nil {
			return err
		}
		if err := persistPlan(ctx, todo.ID, saved, plan, u.newID, repos); err != nil {
			return err
		}
		return bumpTodoRevision(ctx, repos, &todo, &result)
	})
	if err == nil {
		logTodoStateChange(u.logger, ctx, "todo.work.remove", userID, result.ID, workRef.ID)
	}
	return result, err
}
