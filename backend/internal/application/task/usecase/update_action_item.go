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
	ErrActionItemPatchRequiredFieldNull   = errors.New("required action item field cannot be null")
	ErrActionItemScopeInvalid             = errors.New("action item scope must be current or future")
	ErrActionItemFutureDueDateUnsupported = errors.New("due_date cannot be changed for future occurrences; change recurrence weekdays instead")
)

const (
	actionItemScopeCurrent = "current"
	actionItemScopeFuture  = "future"
)

type UpdateActionItemUseCase struct {
	uow    UOW
	ID     shared.ID
	now    func() time.Time
	logger logging.Logger
}

func NewUpdateActionItemUseCase(uow UOW, logger logging.Logger, ids ...shared.ID) *UpdateActionItemUseCase {
	uc := &UpdateActionItemUseCase{uow: uow, now: time.Now, logger: logging.OrNop(logger)}
	if len(ids) > 0 {
		uc.ID = ids[0]
	}
	return uc
}

// ExecuteOccurrence edits one saved occurrence or updates series fields immediately for future scope.
func (uc *UpdateActionItemUseCase) ExecuteOccurrence(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.ActionItemID, occurrenceDate, scope string, title PatchField[string], description PatchField[string], dueDate PatchField[time.Time]) (output dao.ActionItem, err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "UpdateActionItemUseCase.ExecuteOccurrence", err) }()

	asOf := uc.now()
	if scope == actionItemScopeFuture {
		if dueDate.Present {
			return dao.ActionItem{}, ErrActionItemFutureDueDateUnsupported
		}
		if title.Present && title.Value == nil {
			return dao.ActionItem{}, ErrActionItemPatchRequiredFieldNull
		}
		var result dao.ActionItem
		err := uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
			return withTaskProgressMutationForPermission(ctx, repos, userID, taskID, asOf, shared.ActionItemUpdate(), func() error {
				state, err := loadActionItemOccurrenceWithCapability(ctx, repos, userID, taskID, seriesID, occurrenceDate, asOf, shared.ActionItemUpdate())
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
				item, err := actionItemDomainOccurrence(root, nil, mustParseDate(root.OccurrenceDate), domain.ActionItemID(root.ID), root.Completed, asOf)
				if err != nil {
					return err
				}
				item.Title, item.Description, item.Position = effective, descriptionValue, position
				if dueValue == 0 {
					item.DueDate = time.Time{}
				} else {
					item.DueDate = time.Unix(dueValue, 0).UTC()
				}
				validated, err := domain.NewActionItemWithRecurrence(item)
				if err != nil {
					return err
				}
				if !actionItemIsRecurring(root) {
					result, err = repos.ActionItems().UpdateForOwnedTask(ctx, userID, validated)
					return err
				}
				writer, ok := repos.ActionItems().(actionItemSeriesTemplateWriter)
				if !ok {
					return ErrOccurrenceInactive
				}
				snapshotID, err := generatedActionItemID(uc.ID)
				if err != nil {
					return err
				}
				result, err = writer.UpdateActionItemSeriesTemplate(ctx, userID, taskID, seriesID, snapshotID, validated)
				return err
			})
		})
		return result, err
	}
	if scope != actionItemScopeCurrent {
		return dao.ActionItem{}, ErrActionItemScopeInvalid
	}
	if title.Present && title.Value == nil {
		return dao.ActionItem{}, ErrActionItemPatchRequiredFieldNull
	}
	var result dao.ActionItem
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withTaskProgressMutationForPermissionAndState(ctx, repos, userID, taskID, asOf, shared.ActionItemUpdate(), func(_ dao.Task, projectBefore taskProjectMutationSnapshot) error {
			state, err := loadActionItemOccurrenceWithCapability(ctx, repos, userID, taskID, seriesID, occurrenceDate, asOf, shared.ActionItemUpdate())
			if err != nil {
				return err
			}
			state.projectDone = projectBefore.State.Status == "done"
			if projectDoneSuppressesActionItemOccurrence(state) {
				return ErrOccurrenceInactive
			}
			var id domain.ActionItemID
			completed := false
			if state.current != nil {
				id = domain.ActionItemID(state.current.ID)
				completed = state.current.Completed
			} else if !actionItemIsRecurring(state.root) {
				id = domain.ActionItemID(state.root.ID)
			} else {
				id, err = generatedActionItemID(uc.ID)
				if err != nil {
					return err
				}
			}
			item, err := actionItemDomainOccurrence(state.root, state.current, state.date, id, completed, asOf)
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
			validated, err := domain.NewActionItemWithRecurrence(item)
			if err != nil {
				return err
			}
			if !actionItemIsRecurring(state.root) {
				result, err = repos.ActionItems().UpdateForOwnedTask(ctx, userID, validated)
				return err
			}
			writer, err := requireActionItemOverride(repos.ActionItems())
			if err != nil {
				return err
			}
			savedID, err := writer.UpsertActionItemOverride(ctx, userID, validated)
			if err != nil {
				return err
			}
			result = actionItemDAOFromOccurrence(state.root, validated, savedID)
			return nil
		})
	})
	return result, err

}

func (uc *UpdateActionItemUseCase) Execute(
	ctx context.Context,
	userID domain.UserID,
	taskID domain.TaskID,
	actionItemID domain.ActionItemID,
	scope string,
	title PatchField[string],
	description PatchField[string],
	dueDate PatchField[time.Time],
) (dao.ActionItem, error) {
	var row dao.ActionItem
	if err := uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		var err error
		row, err = getActionItemForCommand(ctx, repos.ActionItems(), userID, taskID, actionItemID, shared.ActionItemUpdate())
		return err
	}); err != nil {
		logUnexpectedTaskFailure(uc.logger, ctx, "action_item.update.load", err)
		return dao.ActionItem{}, err
	}
	if row.ID != string(actionItemID) || row.TaskID != string(taskID) {
		return dao.ActionItem{}, ErrActionItemNotFound
	}
	return uc.ExecuteOccurrence(ctx, userID, taskID, domain.ActionItemID(row.SeriesID), row.OccurrenceDate, scope, title, description, dueDate)
}
