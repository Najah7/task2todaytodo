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

var ErrTodoItemPositionOutOfRange = errors.New("todo item position is out of range")

type ReorderTodoItemUseCase struct {
	uow    UOW
	ID     shared.ID
	logger logging.Logger
}

func NewReorderTodoItemUseCase(uow UOW, logger logging.Logger, ids ...shared.ID) *ReorderTodoItemUseCase {
	uc := &ReorderTodoItemUseCase{uow: uow, logger: logging.OrNop(logger)}
	if len(ids) > 0 {
		uc.ID = ids[0]
	}
	return uc
}

func (uc *ReorderTodoItemUseCase) ExecuteOccurrence(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.TodoItemID, occurrenceDate string, position int) (output dao.TodoItem, err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "ReorderTodoItemUseCase.ExecuteOccurrence", err) }()

	if position < 0 {
		return dao.TodoItem{}, ErrTodoItemPositionOutOfRange
	}
	asOf := time.Now()
	var result dao.TodoItem
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		mutate := func(_ dao.Task, projectBefore taskProjectMutationSnapshot) error {
			state, err := loadTodoOccurrenceWithCapability(ctx, repos, userID, taskID, seriesID, occurrenceDate, asOf, shared.TodoItemUpdate())
			if err != nil {
				return err
			}
			state.projectDone = projectBefore.State.Status == "done"
			if projectDoneSuppressesTodoOccurrence(state) {
				return ErrOccurrenceInactive
			}
			rows, err := listTodoOccurrenceCommandProjection(ctx, repos.TodoItems(), userID, taskID, shared.TodoItemUpdate())
			if err != nil {
				return err
			}
			rootIDs := make(map[string]bool)
			for _, row := range rows {
				if row.ID == row.SeriesID {
					rootIDs[row.ID] = true
				}
			}
			skipped := make(map[string]map[string]bool)
			for rootID := range rootIDs {
				if store, ok := repos.TodoItems().(todoSkippedOccurrenceCommandStore); ok {
					dates, err := store.ListTodoItemSkippedOccurrencesForCapability(ctx, userID, taskID, domain.TodoItemID(rootID), shared.TodoItemUpdate())
					if err != nil {
						return err
					}
					skipped[rootID] = make(map[string]bool)
					for _, value := range dates {
						skipped[rootID][time.Unix(value, 0).UTC().Format("2006-01-02")] = true
					}
				} else if store, ok := repos.TodoItems().(todoSkippedOccurrenceStore); ok {
					dates, err := store.ListTodoItemSkippedOccurrences(ctx, userID, taskID, domain.TodoItemID(rootID))
					if err != nil {
						return err
					}
					skipped[rootID] = make(map[string]bool)
					for _, value := range dates {
						skipped[rootID][time.Unix(value, 0).UTC().Format("2006-01-02")] = true
					}
				}
			}
			request := CursorPageRequest{Size: len(rootIDs) + 1, FromDate: state.date.Format("2006-01-02"), AsOf: asOf}
			visible, err := expandTodoItemRowsWithProjectState(rows, request, request.AsOf, false, state.projectDone, skipped)
			if err != nil {
				return err
			}
			day := make([]dao.TodoItem, 0)
			for _, item := range visible {
				if item.OccurrenceDate == state.date.Format("2006-01-02") {
					day = append(day, item)
				}
			}
			if position >= len(day) {
				return ErrTodoItemPositionOutOfRange
			}
			target := -1
			for i, item := range day {
				if item.SeriesID == string(seriesID) && item.OccurrenceDate == state.date.Format("2006-01-02") {
					target = i
					break
				}
			}
			if target < 0 {
				return ErrOccurrenceNotFound
			}
			selected := day[target]
			if target < position {
				copy(day[target:position], day[target+1:position+1])
			} else if target > position {
				copy(day[position+1:target+1], day[position:target])
			}
			day[position] = selected
			writer, err := requireTodoOverride(repos.TodoItems())
			if err != nil {
				return err
			}
			for index, row := range day {
				if row.Position == index {
					if row.SeriesID == string(seriesID) {
						result = row
					}
					continue
				}
				currentState, err := loadTodoOccurrenceWithCapability(ctx, repos, userID, taskID, domain.TodoItemID(row.SeriesID), row.OccurrenceDate, asOf, shared.TodoItemUpdate())
				if err != nil {
					return err
				}
				var id domain.TodoItemID
				completed := false
				if currentState.current != nil {
					id = domain.TodoItemID(currentState.current.ID)
					completed = currentState.current.Completed
				} else {
					id, err = generatedTodoItemID(uc.ID)
					if err != nil {
						return err
					}
				}
				item, err := todoDomainOccurrence(currentState.root, currentState.current, currentState.date, id, completed, asOf)
				if err != nil {
					return err
				}
				item.Position = index
				validated, err := domain.NewTodoItemWithRecurrence(item)
				if err != nil {
					return err
				}
				savedID, err := writer.UpsertTodoItemOverride(ctx, userID, validated)
				if err != nil {
					return err
				}
				if row.SeriesID == string(seriesID) {
					result = todoDAOFromOccurrence(currentState.root, validated, savedID)
				}
			}
			return nil
		}
		return withTaskProgressMutationForPermissionAndState(ctx, repos, userID, taskID, asOf, shared.TodoItemUpdate(), mutate)
	})
	return result, err

}

func (uc *ReorderTodoItemUseCase) Execute(
	ctx context.Context,
	userID domain.UserID,
	taskID domain.TaskID,
	todoItemID domain.TodoItemID,
	position int,
) (dao.TodoItem, error) {
	var current dao.TodoItem
	if err := uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		var err error
		current, err = getTodoItemForCommand(ctx, repos.TodoItems(), userID, taskID, todoItemID, shared.TodoItemUpdate())
		return err
	}); err != nil {
		logUnexpectedTaskFailure(uc.logger, ctx, "todo_item.reorder.load", err)
		return dao.TodoItem{}, err
	}
	if current.ID != string(todoItemID) || current.Deleted {
		return dao.TodoItem{}, ErrTodoItemNotFound
	}
	return uc.ExecuteOccurrence(ctx, userID, taskID, domain.TodoItemID(current.SeriesID), current.OccurrenceDate, position)
}
