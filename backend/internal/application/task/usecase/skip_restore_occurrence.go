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

type SkipTodoItemUseCase struct {
	uow    UOW
	logger logging.Logger
}
type RestoreTodoItemUseCase struct {
	uow    UOW
	logger logging.Logger
}

func NewSkipTodoItemUseCase(uow UOW, logger logging.Logger) *SkipTodoItemUseCase {
	return &SkipTodoItemUseCase{logger: logging.OrNop(logger), uow: uow}
}
func NewRestoreTodoItemUseCase(uow UOW, logger logging.Logger) *RestoreTodoItemUseCase {
	return &RestoreTodoItemUseCase{logger: logging.OrNop(logger), uow: uow}
}
func (uc *SkipTodoItemUseCase) Execute(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.TodoItemID, occurrenceDate string) (err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "SkipTodoItemUseCase.Execute", err) }()

	asOf := time.Now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withTaskProgressMutationForPermissionAndState(ctx, repos, userID, taskID, asOf, shared.OccurrenceUpdate(), func(_ dao.Task, projectBefore taskProjectMutationSnapshot) error {
			state, err := loadTodoOccurrenceWithCapability(ctx, repos, userID, taskID, seriesID, occurrenceDate, asOf, shared.OccurrenceUpdate())
			if errors.Is(err, ErrOccurrenceInactive) && state.skipped {
				return nil
			}
			if err != nil {
				return err
			}
			if state.root.IntervalWeeks == domain.OnceIntervalWeeks {
				return ErrOccurrenceInactive
			}
			if state.current != nil && state.current.Completed {
				return ErrOccurrenceCompleted
			}
			if projectBefore.State.Status == "done" && !state.saved {
				return ErrOccurrenceInactive
			}
			store, ok := repos.TodoItems().(todoSkippedOccurrenceStore)
			if !ok {
				return ErrOccurrenceInactive
			}
			return store.SetTodoItemSkippedOccurrence(ctx, userID, taskID, seriesID, skippedDate(state.date), true)
		})
	})
	if err == nil {
		logTaskStateChange(uc.logger, ctx, "todo_item.skip_occurrence", userID, taskID, string(seriesID))
	}
	return err
}

func (uc *RestoreTodoItemUseCase) Execute(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.TodoItemID, occurrenceDate string) (err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "RestoreTodoItemUseCase.Execute", err) }()

	asOf := time.Now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withTaskProgressMutationForPermission(ctx, repos, userID, taskID, asOf, shared.OccurrenceUpdate(), func() error {
			state, err := loadTodoOccurrenceWithCapability(ctx, repos, userID, taskID, seriesID, occurrenceDate, asOf, shared.OccurrenceUpdate())
			if errors.Is(err, ErrOccurrenceInactive) && state.skipped {
				err = nil
			}
			if err != nil {
				return err
			}
			if state.root.IntervalWeeks == domain.OnceIntervalWeeks {
				return ErrOccurrenceInactive
			}
			store, ok := repos.TodoItems().(todoSkippedOccurrenceStore)
			if !ok {
				return ErrOccurrenceNotFound
			}
			return store.SetTodoItemSkippedOccurrence(ctx, userID, taskID, seriesID, skippedDate(state.date), false)
		})
	})
	if err == nil {
		logTaskStateChange(uc.logger, ctx, "todo_item.restore_occurrence", userID, taskID, string(seriesID))
	}
	return err
}
