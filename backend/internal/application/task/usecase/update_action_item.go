package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

var ErrActionItemPatchRequiredFieldNull = errors.New("required action item field cannot be null")
var ErrActionItemActualMinutesRequiresCompletion = errors.New("actual minutes can only be set on completed action items")

type ActionItemPlanningPatch struct {
	EstimatedMinutes        PatchField[int]
	ActualMinutes           PatchField[int]
	Priority                PatchField[string]
	ConfirmTodoRegeneration bool
}

type UpdateActionItemUseCase struct {
	uow    UOW
	logger telemetry.Logger
}

func NewUpdateActionItemUseCase(uow UOW, logger telemetry.Logger) *UpdateActionItemUseCase {
	return &UpdateActionItemUseCase{uow: uow, logger: telemetry.OrNop(logger)}
}

func (uc *UpdateActionItemUseCase) Execute(
	ctx context.Context,
	userID domain.UserID,
	taskID domain.TaskID,
	itemID domain.ActionItemID,
	title PatchField[string],
	description PatchField[string],
	dueDate PatchField[time.Time],
	planning ...ActionItemPlanningPatch,
) (output dao.ActionItem, err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "UpdateActionItemUseCase.Execute", err) }()
	if title.Present && title.Value == nil {
		return dao.ActionItem{}, ErrActionItemPatchRequiredFieldNull
	}
	var patch ActionItemPlanningPatch
	if len(planning) > 0 {
		patch = planning[0]
	}
	asOf := time.Now()
	var result dao.ActionItem
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withTaskProgressMutationForPermissionAndState(ctx, repos, userID, taskID, asOf, shared.ActionItemUpdate(), func(task dao.Task, _ taskProjectMutationSnapshot) error {
			current, err := repos.ActionItems().GetForCommand(ctx, userID, taskID, itemID, shared.ActionItemUpdate())
			if err != nil {
				return err
			}
			item, err := actionItemDomainFromDAO(current)
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
			if patch.EstimatedMinutes.Present {
				if patch.EstimatedMinutes.Value == nil {
					return domain.ErrActionItemEstimatedMinutesInvalid
				}
				item.EstimatedMinutes = nil
				if patch.EstimatedMinutes.Value != nil {
					value := *patch.EstimatedMinutes.Value
					item.EstimatedMinutes = &value
				}
			}
			estimateChanged := patch.EstimatedMinutes.Present && (current.EstimatedMinutes == nil || item.EstimatedMinutes == nil || *current.EstimatedMinutes != *item.EstimatedMinutes)
			todayHasItem := false
			assigneeID := domain.UserID(task.AssigneeID)
			if assigneeID == "" {
				assigneeID = userID
			}
			if estimateChanged {
				if current.Completed {
					return ErrActionItemStarted
				}
				started, err := repos.ActionItemTodos().HasCompletedFrame(ctx, itemID)
				if err != nil {
					return err
				}
				if started {
					return ErrActionItemStarted
				}
				todayHasItem, err = repos.ActionItemTodos().HasTodayActionItem(ctx, assigneeID, itemID)
				if err != nil {
					return err
				}
				if todayHasItem && !patch.ConfirmTodoRegeneration {
					return ErrTodoRegenerationRequired
				}
			}
			if patch.ActualMinutes.Present {
				if !current.Completed {
					return ErrActionItemActualMinutesRequiresCompletion
				}
				item.ActualMinutes = nil
				if patch.ActualMinutes.Value != nil {
					value := *patch.ActualMinutes.Value
					item.ActualMinutes = &value
				}
			}
			if patch.Priority.Present {
				if patch.Priority.Value == nil {
					return domain.ErrTaskPriorityEmpty
				}
				item.Priority, err = domain.NewTaskPriority(*patch.Priority.Value)
				if err != nil {
					return err
				}
			}
			if err := item.Validate(); err != nil {
				return err
			}
			result, err = repos.ActionItems().UpdateForOwnedTask(ctx, userID, item)
			if err == nil && estimateChanged && todayHasItem {
				err = repos.ActionItemTodos().RegenerateToday(ctx, assigneeID)
			}
			return err
		})
	})
	return result, err
}

func actionItemDomainFromDAO(item dao.ActionItem) (domain.ActionItem, error) {
	var dueDate time.Time
	if item.DueDate != 0 {
		dueDate = time.Unix(item.DueDate, 0).UTC()
	}
	createdAt := time.Unix(item.CreatedAt, 0).UTC()
	updatedAt := time.Unix(item.UpdatedAt, 0).UTC()
	priority := domain.TaskPriority{}
	var err error
	if item.Priority.Value != "" {
		priority, err = domain.NewTaskPriority(item.Priority.Value)
		if err != nil {
			return domain.ActionItem{}, err
		}
	}
	return domain.NewExistingActionItem(domain.ActionItemID(item.ID), domain.TaskID(item.TaskID), item.Title, item.Description, dueDate, item.Completed, item.Position, createdAt, updatedAt, domain.ActionItemPlanning{
		EstimatedMinutes: item.EstimatedMinutes,
		ActualMinutes:    item.ActualMinutes,
		Priority:         priority,
	})
}
