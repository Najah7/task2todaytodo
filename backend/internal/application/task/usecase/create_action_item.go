package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type CreateActionItemInput struct {
	ID                      domain.ActionItemID
	UserID                  domain.UserID
	TaskID                  domain.TaskID
	Title                   string
	Description             string
	DueDate                 time.Time
	EstimatedMinutes        *int
	Priority                string
	ConfirmTodoRegeneration bool
}

type CreateActionItemUseCase struct {
	uow    UOW
	logger telemetry.Logger
}

func NewCreateActionItemUseCase(uow UOW, logger telemetry.Logger) *CreateActionItemUseCase {
	return &CreateActionItemUseCase{logger: telemetry.OrNop(logger), uow: uow}
}

func (uc *CreateActionItemUseCase) Execute(ctx context.Context, input CreateActionItemInput) (output dao.ActionItem, err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "CreateActionItemUseCase.Execute", err) }()

	var created dao.ActionItem
	asOf := time.Now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withTaskProgressMutationForPermissionAndState(ctx, repos, input.UserID, input.TaskID, asOf, shared.ActionItemCreate(), func(task dao.Task, _ taskProjectMutationSnapshot) error {
			childCount, err := repos.ActionItems().CountForOwnedTask(ctx, input.UserID, input.TaskID, shared.ActionItemCreate())
			if err != nil {
				return err
			}
			regenerate := false
			if childCount == 0 {
				started, err := repos.TaskTodos().HasCompletedTaskFrame(ctx, input.TaskID)
				if err != nil {
					return err
				}
				if started || task.Status.Value == "done" {
					return ErrCannotAddActionItemToStartedTask
				}
				assignee := domain.UserID(task.AssigneeID)
				if assignee == "" {
					assignee = input.UserID
				}
				regenerate, err = repos.TaskTodos().HasTodayTaskFrame(ctx, assignee, input.TaskID)
				if err != nil {
					return err
				}
				if regenerate && !input.ConfirmTodoRegeneration {
					return ErrTodoRegenerationRequired
				}
			}
			priorityValue := input.Priority
			if priorityValue == "" {
				priorityValue = task.Priority.Value
			}
			priority := domain.TaskPriority{}
			if priorityValue != "" {
				priority, err = domain.NewTaskPriority(priorityValue)
				if err != nil {
					return err
				}
			}
			item, err := domain.NewActionItemWithDetails(input.ID, input.TaskID, input.Title, input.Description, input.DueDate, false, 0, domain.ActionItemPlanning{
				EstimatedMinutes: input.EstimatedMinutes,
				Priority:         priority,
			})
			if err != nil {
				return err
			}
			created, err = repos.ActionItems().CreateForOwnedTask(ctx, input.UserID, item, true)
			if err == nil && regenerate {
				assignee := domain.UserID(task.AssigneeID)
				if assignee == "" {
					assignee = input.UserID
				}
				err = repos.TaskTodos().RegenerateToday(ctx, assignee)
			}
			return err
		})
	})
	if err != nil {
		return dao.ActionItem{}, err
	}
	return created, nil
}
