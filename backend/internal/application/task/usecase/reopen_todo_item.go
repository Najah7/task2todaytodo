package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type reopenTodoItemUOW interface {
	Do(ctx context.Context, fn func(context.Context, Repositories) error) error
}

type ReopenTodoItemUseCase struct {
	uow    reopenTodoItemUOW
	logger logging.Logger
}

func NewReopenTodoItemUseCase(uow reopenTodoItemUOW, logger logging.Logger) *ReopenTodoItemUseCase {
	return &ReopenTodoItemUseCase{logger: logging.OrNop(logger), uow: uow}
}

func (uc *ReopenTodoItemUseCase) Execute(ctx context.Context, userID domain.UserID, taskID domain.TaskID, todoItemID domain.TodoItemID) (err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "ReopenTodoItemUseCase.Execute", err) }()

	asOf := time.Now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withTaskProgressMutationForPermission(ctx, repos, userID, taskID, asOf, shared.TodoItemUpdate(), func() error {
			return repos.TodoItems().UncheckForOwnedTask(ctx, userID, taskID, todoItemID)
		})
	})
	if err == nil {
		logTaskStateChange(uc.logger, ctx, "todo_item.reopen", userID, taskID, string(todoItemID))
	}
	return err
}

func (uc *ReopenTodoItemUseCase) ExecuteOccurrence(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.TodoItemID, occurrenceDate string) (err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "ReopenTodoItemUseCase.ExecuteOccurrence", err) }()

	asOf := time.Now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withTaskProgressMutationForPermissionAndState(ctx, repos, userID, taskID, asOf, shared.TodoItemUpdate(), func(_ dao.Task, projectBefore taskProjectMutationSnapshot) error {
			state, err := loadTodoOccurrenceWithCapability(ctx, repos, userID, taskID, seriesID, occurrenceDate, asOf, shared.TodoItemUpdate())
			if err != nil {
				return err
			}
			if projectBefore.State.Status == "done" && !state.saved {
				return ErrOccurrenceInactive
			}
			if !todoItemIsRecurring(state.root) {
				return repos.TodoItems().UncheckForOwnedTask(ctx, userID, taskID, domain.TodoItemID(state.root.ID))
			}
			if state.current == nil || !state.current.Completed {
				return nil
			}
			return repos.TodoItems().UncheckForOwnedTask(ctx, userID, taskID, domain.TodoItemID(state.current.ID))
		})
	})
	if err == nil {
		logTaskStateChange(uc.logger, ctx, "todo_item.reopen_occurrence", userID, taskID, string(seriesID))
	}
	return err
}
