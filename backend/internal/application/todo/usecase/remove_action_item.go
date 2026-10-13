package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/todo/dao"
	"github.com/Najah7/task2todaytodo/internal/application/todo/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type RemoveActionItemUseCase struct {
	uow    UOW
	logger telemetry.Logger
}

func NewRemoveActionItemUseCase(uow UOW, _ func() string, loggers ...telemetry.Logger) *RemoveActionItemUseCase {
	var logger telemetry.Logger
	if len(loggers) > 0 {
		logger = loggers[0]
	}
	return &RemoveActionItemUseCase{uow: uow, logger: telemetry.OrNop(logger)}
}

func (u *RemoveActionItemUseCase) Execute(ctx context.Context, userID, frameID string, expectedRevision int32) (result dao.Todo, err error) {
	defer func() { logUnexpectedTodoFailure(u.logger, ctx, "RemoveActionItemUseCase.Execute", err) }()
	var removedID string
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
			if items[index].ID == frameID && domain.FrameKind(items[index].Kind).IsWork() {
				target = &items[index]
				break
			}
		}
		if target == nil {
			return ErrFrameNotFound
		}
		if target.Completed {
			return ErrFrameCompleted
		}
		ref := itemRef(*target)
		workSources, err := repos.Sources().ReadWorkSources(ctx, userID, todo.Date, todo.Timezone, []dao.WorkSourceRef{ref})
		if err != nil {
			return err
		}
		source, ok := sourceByRef(workSources, ref)
		if !ok {
			return ErrPermissionDenied
		}
		if !source.CanRemoveFromToday {
			return ErrPermissionDenied
		}
		if err := repos.Items().DeleteByID(ctx, todo.ID, target.ID); err != nil {
			return err
		}
		models := make([]domain.TodoItem, 0, len(items)-1)
		for _, item := range items {
			if item.ID == target.ID {
				continue
			}
			model, err := restoredItem(item)
			if err != nil {
				return err
			}
			model.Position = len(models)
			models = append(models, model)
		}
		if err := repos.Items().SavePositions(ctx, todo.ID, models); err != nil {
			return err
		}
		if err := bumpTodoRevision(ctx, repos, &todo, &result); err != nil {
			return err
		}
		removedID = ref.ID
		return nil
	})
	if err == nil {
		logTodoStateChange(u.logger, ctx, "todo.work.remove", userID, result.ID, removedID)
	}
	return result, err
}
