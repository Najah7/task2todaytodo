package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type completeTodoItemUOW interface {
	Do(ctx context.Context, fn func(context.Context, Repositories) error) error
}

type CompleteTodoItemUseCase struct {
	uow    completeTodoItemUOW
	ID     shared.ID
	logger logging.Logger
}

func NewCompleteTodoItemUseCase(uow completeTodoItemUOW, logger logging.Logger, ids ...shared.ID) *CompleteTodoItemUseCase {
	var ID shared.ID
	if len(ids) > 0 {
		ID = ids[0]
	}
	return &CompleteTodoItemUseCase{logger: logging.OrNop(logger), uow: uow, ID: ID}
}

func (uc *CompleteTodoItemUseCase) ExecuteOccurrence(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.TodoItemID, occurrenceDate string) (err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "CompleteTodoItemUseCase.ExecuteOccurrence", err) }()

	asOf := time.Now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withTaskProgressMutationForPermissionAndState(ctx, repos, userID, taskID, asOf, shared.TodoItemUpdate(), func(_ dao.Task, projectBefore taskProjectMutationSnapshot) error {
			state, err := loadTodoOccurrenceWithCapability(ctx, repos, userID, taskID, seriesID, occurrenceDate, asOf, shared.TodoItemUpdate())
			if err != nil {
				return err
			}
			state.projectDone = projectBefore.State.Status == "done"
			if projectDoneSuppressesTodoOccurrence(state) {
				return ErrOccurrenceInactive
			}
			if !todoItemIsRecurring(state.root) {
				return repos.TodoItems().CheckForOwnedTask(ctx, userID, taskID, domain.TodoItemID(state.root.ID))
			}
			if state.current != nil {
				return repos.TodoItems().CheckForOwnedTask(ctx, userID, taskID, domain.TodoItemID(state.current.ID))
			}
			id, err := generatedTodoItemID(uc.ID)
			if err != nil {
				return err
			}
			item, err := todoDomainOccurrence(state.root, nil, state.date, id, true, asOf)
			if err != nil {
				return err
			}
			writer, err := requireTodoOverride(repos.TodoItems())
			if err != nil {
				return err
			}
			_, err = writer.UpsertTodoItemOverride(ctx, userID, item)
			return err
		})
	})
	if err == nil {
		logTaskStateChange(uc.logger, ctx, "todo_item.complete_occurrence", userID, taskID, string(seriesID))
	}
	return err
}

func (uc *CompleteTodoItemUseCase) Execute(ctx context.Context, userID domain.UserID, taskID domain.TaskID, todoItemID domain.TodoItemID) (err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "CompleteTodoItemUseCase.Execute", err) }()

	asOf := time.Now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withTaskProgressMutationForPermission(ctx, repos, userID, taskID, asOf, shared.TodoItemUpdate(), func() error {
			return repos.TodoItems().CheckForOwnedTask(ctx, userID, taskID, todoItemID)
		})
	})
	if err == nil {
		logTaskStateChange(uc.logger, ctx, "todo_item.complete", userID, taskID, string(todoItemID))
	}
	return err
}
