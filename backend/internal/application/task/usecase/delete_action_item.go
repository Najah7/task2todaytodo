package usecase

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

var ErrActionItemCompleted = errors.New("completed action item cannot be deleted")

type DeleteActionItemOptions struct {
	ConfirmTodoRegeneration bool
	ManualEstimatedMinutes  *int
	ManualEstimatePresent   bool
	ExpectedTaskRevision    *int32
}

type DeleteActionItemUseCase struct {
	uow    UOW
	logger telemetry.Logger
}

func NewDeleteActionItemUseCase(uow UOW, logger telemetry.Logger) *DeleteActionItemUseCase {
	return &DeleteActionItemUseCase{logger: telemetry.OrNop(logger), uow: uow}
}

func (uc *DeleteActionItemUseCase) Execute(ctx context.Context, userID domain.UserID, taskID domain.TaskID, actionItemID domain.ActionItemID, confirm ...bool) (err error) {
	options := DeleteActionItemOptions{}
	if len(confirm) > 0 {
		options.ConfirmTodoRegeneration = confirm[0]
	}
	return uc.ExecuteWithOptions(ctx, userID, taskID, actionItemID, options)
}

func (uc *DeleteActionItemUseCase) ExecuteWithOptions(ctx context.Context, userID domain.UserID, taskID domain.TaskID, actionItemID domain.ActionItemID, options DeleteActionItemOptions) (err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "DeleteActionItemUseCase.Execute", err) }()
	asOf := time.Now()
	return uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		confirmed := options.ConfirmTodoRegeneration
		return withTaskProgressMutationForPermissionAndState(ctx, repos, userID, taskID, asOf, shared.ActionItemDelete(), func(task dao.Task, _ taskProjectMutationSnapshot) error {
			if options.ExpectedTaskRevision != nil && task.Revision != *options.ExpectedTaskRevision {
				return ErrRevisionConflict
			}
			repository := repos.ActionItems()
			item, err := repository.GetForCommand(ctx, userID, taskID, actionItemID, shared.ActionItemDelete())
			if err != nil {
				return err
			}
			if item.Deleted {
				return ErrActionItemNotFound
			}
			if item.Completed {
				return ErrActionItemCompleted
			}
			childCount, err := repository.CountForOwnedTask(ctx, userID, taskID, shared.ActionItemDelete())
			if err != nil {
				return err
			}
			regenerate := childCount == 1
			assignee := domain.UserID(task.AssigneeID)
			if assignee == "" {
				assignee = userID
			}
			var fallbackEstimate *int
			if regenerate {
				fallbackEstimate = copyOptionalInt(task.ManualEstimatedMinutes)
				if options.ManualEstimatePresent {
					fallbackEstimate = copyOptionalInt(options.ManualEstimatedMinutes)
				}
				if fallbackEstimate == nil || *fallbackEstimate < 1 || int64(*fallbackEstimate) > math.MaxInt32 {
					return ErrTaskManualEstimateRequired
				}
				estimateChanged := options.ManualEstimatePresent && !sameOptionalInt(task.ManualEstimatedMinutes, options.ManualEstimatedMinutes)
				if estimateChanged {
					started, err := repos.TaskTodos().HasCompletedTaskFrame(ctx, taskID)
					if err != nil {
						return err
					}
					if started || task.Status.Value == "done" {
						return ErrTaskStarted
					}
				}
				hasToday, err := repos.TaskTodos().HasTodayTaskWork(ctx, assignee, taskID)
				if err != nil {
					return err
				}
				regenerate = hasToday
				if hasToday && !confirmed {
					return ErrTodoRegenerationRequired
				}
			}
			if err := repository.DeleteForOwnedTask(ctx, userID, taskID, actionItemID); err != nil {
				return err
			}
			if childCount == 1 && (options.ManualEstimatePresent || task.ManualEstimatedMinutes == nil) {
				current, err := repos.Tasks().LockByUserIDWithPermission(ctx, userID, taskID, shared.TaskUpdate())
				if err != nil {
					return err
				}
				updated, err := taskFromDAO(current)
				if err != nil {
					return err
				}
				updated.ManualEstimatedMinutes = copyOptionalInt(fallbackEstimate)
				validated, err := domain.NewExistingTask(updated.ID, updated.UserID, updated.ProjectID, updated.Title, updated.Description, updated.DueDate, updated.ManualEstimatedMinutes, updated.ActualMinutes, updated.Progress, updated.Priority, updated.Status, updated.AssigneeID, updated.CreatedAt, updated.UpdatedAt)
				if err != nil {
					return err
				}
				if _, err = repos.Tasks().UpdateByUserID(ctx, userID, validated, current.Revision); err != nil {
					return err
				}
			}
			if regenerate {
				return repos.TaskTodos().RegenerateToday(ctx, assignee)
			}
			return nil
		})
	})
}
