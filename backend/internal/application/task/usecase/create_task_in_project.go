package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type CreateTaskInProjectInput struct {
	ID               domain.TaskID
	UserID           domain.UserID
	ProjectID        domain.ProjectID
	Title            string
	Description      string
	DueDate          time.Time
	EstimatedMinutes *int
	Priority         string
}

type CreateTaskInProjectUseCase struct {
	uow    UOW
	logger logging.Logger
}

func NewCreateTaskInProjectUseCase(uow UOW, logger logging.Logger) *CreateTaskInProjectUseCase {
	return &CreateTaskInProjectUseCase{logger: logging.OrNop(logger), uow: uow}
}

func (uc *CreateTaskInProjectUseCase) Execute(ctx context.Context, input CreateTaskInProjectInput) (output dao.Task, err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "CreateTaskInProjectUseCase.Execute", err) }()

	var created dao.Task
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		project, err := repos.Projects().LockByUserIDWithPermission(ctx, input.UserID, input.ProjectID, shared.TaskCreate())
		if err != nil {
			return err
		}

		priorityValue := input.Priority
		if priorityValue == "" {
			priorityValue = project.Priority.Value
		}
		priority, err := domain.NewTaskPriority(priorityValue)
		if err != nil {
			return err
		}
		status, err := domain.NewTaskStatus("open")
		if err != nil {
			return err
		}
		task, err := domain.NewTaskWithDetails(
			input.ID,
			input.UserID,
			domain.ProjectID(project.ID),
			input.Title,
			input.Description,
			input.DueDate,
			input.EstimatedMinutes,
			nil,
			0,
			priority,
			status,
		)
		if err != nil {
			return err
		}

		task.UserID = domain.UserID(project.UserID)
		task, err = task.AssignTo(domain.UserID(project.UserID))
		if err != nil {
			return err
		}
		created, err = repos.Tasks().CreateInProject(ctx, input.UserID, task)
		return err
	})
	if err != nil {
		return dao.Task{}, err
	}
	return created, nil

}
