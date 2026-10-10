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
	asOf := time.Now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		project, err := repos.TaskProjects().LockProjectByUserIDWithPermission(ctx, string(input.UserID), string(input.ProjectID), shared.TaskCreate())
		if err != nil {
			return err
		}
		lifecycle := repos.ProjectLifecycle()
		if lifecycle == nil {
			return ErrProjectLifecycleUnavailable
		}
		before, err := lifecycle.CaptureWorkState(ctx, project.ID, asOf)
		if err != nil {
			return err
		}

		priorityValue := input.Priority
		if priorityValue == "" {
			priorityValue = project.DefaultPriority
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

		task.UserID = domain.UserID(project.OwnerID)
		task, err = task.AssignTo(domain.UserID(project.OwnerID))
		if err != nil {
			return err
		}
		created, err = repos.Tasks().CreateInProject(ctx, input.UserID, task)
		if err != nil {
			return err
		}
		created.CanUpdate, err = repos.Tasks().HasPermission(ctx, input.UserID, domain.TaskID(created.ID), shared.TaskUpdate())
		if err != nil {
			return err
		}
		if err := lifecycle.ReconcileWorkState(ctx, string(input.UserID), project.ID, before, asOf); err != nil {
			return err
		}
		rows, err := EnrichTasksInRepositories(ctx, repos, input.UserID, []dao.Task{created}, asOf)
		if err != nil {
			return err
		}
		created = rows[0]
		return nil
	})
	if err != nil {
		return dao.Task{}, err
	}
	return created, nil

}
