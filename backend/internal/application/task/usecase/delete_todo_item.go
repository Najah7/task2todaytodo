package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type deleteTodoItemUOW interface {
	Do(ctx context.Context, fn func(context.Context, Repositories) error) error
}

type DeleteTodoItemUseCase struct {
	uow    deleteTodoItemUOW
	logger logging.Logger
}

func NewDeleteTodoItemUseCase(uow deleteTodoItemUOW, logger logging.Logger) *DeleteTodoItemUseCase {
	return &DeleteTodoItemUseCase{logger: logging.OrNop(logger), uow: uow}
}

func (uc *DeleteTodoItemUseCase) Execute(ctx context.Context, userID domain.UserID, taskID domain.TaskID, todoItemID domain.TodoItemID) (err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "DeleteTodoItemUseCase.Execute", err) }()

	asOf := time.Now()
	return uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withTaskProgressMutationForPermission(ctx, repos, userID, taskID, asOf, shared.TodoItemDelete(), func() error {
			todoItems := repos.TodoItems()
			item, err := getTodoItemForCommand(ctx, todoItems, userID, taskID, todoItemID, shared.TodoItemDelete())
			if err != nil {
				return err
			}

			if item.RepeatState == repeatStateOneOff && item.Completed {
				return ErrOccurrenceCompleted
			}
			if item.Deleted {
				return ErrOccurrenceInactive
			}
			// Deleting the root suppresses future projections while retaining its
			// repeat metadata for saved occurrence history.
			if item.SeriesID == item.ID {
				if err := todoItems.TombstoneForOwnedTask(ctx, userID, taskID, todoItemID); err != nil {
					return err
				}
				return nil
			}

			// Keep the deleted occurrence row so this date stays suppressed.
			return todoItems.TombstoneForOwnedTask(ctx, userID, taskID, todoItemID)
		})
	})

}

func (uc *DeleteTodoItemUseCase) DeleteSeries(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.TodoItemID) (err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "DeleteTodoItemUseCase.DeleteSeries", err) }()

	return uc.Execute(ctx, userID, taskID, seriesID)

}
