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

var (
	ErrTodoItemPatchRequiredFieldNull   = errors.New("required todo item field cannot be null")
	ErrTodoItemScopeInvalid             = errors.New("todo item scope must be current or future")
	ErrTodoItemFutureDueDateUnsupported = errors.New("due_date cannot be changed for future occurrences; change recurrence weekdays instead")
)

const (
	todoItemScopeCurrent = "current"
	todoItemScopeFuture  = "future"
)

type UpdateTodoItemUseCase struct {
	uow    UOW
	ID     shared.ID
	now    func() time.Time
	logger logging.Logger
}

func NewUpdateTodoItemUseCase(uow UOW, logger logging.Logger, ids ...shared.ID) *UpdateTodoItemUseCase {
	uc := &UpdateTodoItemUseCase{uow: uow, now: time.Now, logger: logging.OrNop(logger)}
	if len(ids) > 0 {
		uc.ID = ids[0]
	}
	return uc
}

// ExecuteOccurrence edits one saved occurrence or updates series fields immediately for future scope.
func (uc *UpdateTodoItemUseCase) ExecuteOccurrence(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.TodoItemID, occurrenceDate, scope string, title PatchField[string], description PatchField[string], dueDate PatchField[time.Time]) (output dao.TodoItem, err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "UpdateTodoItemUseCase.ExecuteOccurrence", err) }()

	asOf := uc.now()
	if scope == todoItemScopeFuture {
		if dueDate.Present {
			return dao.TodoItem{}, ErrTodoItemFutureDueDateUnsupported
		}
		if title.Present && title.Value == nil {
			return dao.TodoItem{}, ErrTodoItemPatchRequiredFieldNull
		}
		var result dao.TodoItem
		err := uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
			return withTaskProgressMutationForPermission(ctx, repos, userID, taskID, asOf, shared.TodoItemUpdate(), func() error {
				state, err := loadTodoOccurrenceWithCapability(ctx, repos, userID, taskID, seriesID, occurrenceDate, asOf, shared.TodoItemUpdate())
				if err != nil {
					return err
				}
				root := state.root
				effective := root.Title
				descriptionValue := root.Description
				dueValue := root.DueDate
				position := root.Position
				if title.Present {
					effective = *title.Value
				}
				if description.Present {
					descriptionValue = ""
					if description.Value != nil {
						descriptionValue = *description.Value
					}
				}
				item, err := todoDomainOccurrence(root, nil, mustParseDate(root.OccurrenceDate), domain.TodoItemID(root.ID), root.Completed, asOf)
				if err != nil {
					return err
				}
				item.Title, item.Description, item.Position = effective, descriptionValue, position
				if dueValue == 0 {
					item.DueDate = time.Time{}
				} else {
					item.DueDate = time.Unix(dueValue, 0).UTC()
				}
				validated, err := domain.NewTodoItemWithRecurrence(item)
				if err != nil {
					return err
				}
				if !todoItemIsRecurring(root) {
					result, err = repos.TodoItems().UpdateForOwnedTask(ctx, userID, validated)
					return err
				}
				writer, ok := repos.TodoItems().(todoSeriesTemplateWriter)
				if !ok {
					return ErrOccurrenceInactive
				}
				snapshotID, err := generatedTodoItemID(uc.ID)
				if err != nil {
					return err
				}
				result, err = writer.UpdateTodoItemSeriesTemplate(ctx, userID, taskID, seriesID, snapshotID, validated)
				return err
			})
		})
		return result, err
	}
	if scope != todoItemScopeCurrent {
		return dao.TodoItem{}, ErrTodoItemScopeInvalid
	}
	if title.Present && title.Value == nil {
		return dao.TodoItem{}, ErrTodoItemPatchRequiredFieldNull
	}
	var result dao.TodoItem
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
			var id domain.TodoItemID
			completed := false
			if state.current != nil {
				id = domain.TodoItemID(state.current.ID)
				completed = state.current.Completed
			} else if !todoItemIsRecurring(state.root) {
				id = domain.TodoItemID(state.root.ID)
			} else {
				id, err = generatedTodoItemID(uc.ID)
				if err != nil {
					return err
				}
			}
			item, err := todoDomainOccurrence(state.root, state.current, state.date, id, completed, asOf)
			if err != nil {
				return err
			}
			if title.Present {
				item.Title = *title.Value
			}
			if description.Present {
				item.Description = ""
				if description.Value != nil {
					item.Description = *description.Value
				}
			}
			if dueDate.Present {
				item.DueDate = time.Time{}
				if dueDate.Value != nil {
					item.DueDate = *dueDate.Value
				}
			}
			validated, err := domain.NewTodoItemWithRecurrence(item)
			if err != nil {
				return err
			}
			if !todoItemIsRecurring(state.root) {
				result, err = repos.TodoItems().UpdateForOwnedTask(ctx, userID, validated)
				return err
			}
			writer, err := requireTodoOverride(repos.TodoItems())
			if err != nil {
				return err
			}
			savedID, err := writer.UpsertTodoItemOverride(ctx, userID, validated)
			if err != nil {
				return err
			}
			result = todoDAOFromOccurrence(state.root, validated, savedID)
			return nil
		})
	})
	return result, err

}

func (uc *UpdateTodoItemUseCase) Execute(
	ctx context.Context,
	userID domain.UserID,
	taskID domain.TaskID,
	todoItemID domain.TodoItemID,
	scope string,
	title PatchField[string],
	description PatchField[string],
	dueDate PatchField[time.Time],
) (dao.TodoItem, error) {
	var row dao.TodoItem
	if err := uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		var err error
		row, err = getTodoItemForCommand(ctx, repos.TodoItems(), userID, taskID, todoItemID, shared.TodoItemUpdate())
		return err
	}); err != nil {
		logUnexpectedTaskFailure(uc.logger, ctx, "todo_item.update.load", err)
		return dao.TodoItem{}, err
	}
	if row.ID != string(todoItemID) || row.TaskID != string(taskID) {
		return dao.TodoItem{}, ErrTodoItemNotFound
	}
	return uc.ExecuteOccurrence(ctx, userID, taskID, domain.TodoItemID(row.SeriesID), row.OccurrenceDate, scope, title, description, dueDate)
}
