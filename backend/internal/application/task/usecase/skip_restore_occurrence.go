package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
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
type SkipTaskScheduleUseCase struct {
	uow    UOW
	logger logging.Logger
}
type RestoreTaskScheduleUseCase struct {
	uow    UOW
	logger logging.Logger
}

func NewSkipTodoItemUseCase(uow UOW, logger logging.Logger) *SkipTodoItemUseCase {
	return &SkipTodoItemUseCase{logger: logging.OrNop(logger), uow: uow}
}
func NewRestoreTodoItemUseCase(uow UOW, logger logging.Logger) *RestoreTodoItemUseCase {
	return &RestoreTodoItemUseCase{logger: logging.OrNop(logger), uow: uow}
}
func NewSkipTaskScheduleUseCase(uow UOW, logger logging.Logger) *SkipTaskScheduleUseCase {
	return &SkipTaskScheduleUseCase{logger: logging.OrNop(logger), uow: uow}
}
func NewRestoreTaskScheduleUseCase(uow UOW, logger logging.Logger) *RestoreTaskScheduleUseCase {
	return &RestoreTaskScheduleUseCase{logger: logging.OrNop(logger), uow: uow}
}

func (uc *SkipTodoItemUseCase) Execute(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.TodoItemID, occurrenceDate string) (err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "SkipTodoItemUseCase.Execute", err) }()

	asOf := time.Now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withTaskProgressMutationForPermission(ctx, repos, userID, taskID, asOf, shared.OccurrenceUpdate(), func() error {
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

func (uc *SkipTaskScheduleUseCase) Execute(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.TaskScheduleID, occurrenceDate string) (err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "SkipTaskScheduleUseCase.Execute", err) }()

	asOf := time.Now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withTaskProgressMutationForPermission(ctx, repos, userID, taskID, asOf, shared.OccurrenceUpdate(), func() error {
			state, err := loadTaskScheduleOccurrenceWithCapability(ctx, repos, userID, taskID, seriesID, occurrenceDate, asOf, shared.OccurrenceUpdate())
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
			store, ok := repos.TaskSchedules().(scheduleSkippedOccurrenceStore)
			if !ok {
				return ErrOccurrenceInactive
			}
			return store.SetTaskScheduleSkippedOccurrence(ctx, userID, taskID, seriesID, skippedDate(state.date), true)
		})
	})
	if err == nil {
		logTaskStateChange(uc.logger, ctx, "task_schedule.skip_occurrence", userID, taskID, string(seriesID))
	}
	return err
}

func (uc *RestoreTaskScheduleUseCase) Execute(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.TaskScheduleID, occurrenceDate string) (err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "RestoreTaskScheduleUseCase.Execute", err) }()

	asOf := time.Now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withTaskProgressMutationForPermission(ctx, repos, userID, taskID, asOf, shared.OccurrenceUpdate(), func() error {
			state, err := loadTaskScheduleOccurrenceWithCapability(ctx, repos, userID, taskID, seriesID, occurrenceDate, asOf, shared.OccurrenceUpdate())
			if errors.Is(err, ErrOccurrenceInactive) && state.skipped {
				err = nil
			}
			if err != nil {
				return err
			}
			if state.root.IntervalWeeks == domain.OnceIntervalWeeks {
				return ErrOccurrenceInactive
			}
			store, ok := repos.TaskSchedules().(scheduleSkippedOccurrenceStore)
			if !ok {
				return ErrOccurrenceNotFound
			}
			return store.SetTaskScheduleSkippedOccurrence(ctx, userID, taskID, seriesID, skippedDate(state.date), false)
		})
	})
	if err == nil {
		logTaskStateChange(uc.logger, ctx, "task_schedule.restore_occurrence", userID, taskID, string(seriesID))
	}
	return err
}
