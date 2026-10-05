package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type CreateProjectInput struct {
	ID          domain.ProjectID
	UserID      domain.UserID
	Type        string
	Title       string
	Goal        string
	Description string
	Priority    string
	StartDate   *time.Time
	EndDate     *time.Time
}

type createProjectRepository interface {
	Create(ctx context.Context, project domain.Project) (dao.Project, error)
}

type CreateProjectUseCase struct {
	repo   createProjectRepository
	logger logging.Logger
}

func NewCreateProjectUseCase(repo createProjectRepository, logger logging.Logger) *CreateProjectUseCase {
	return &CreateProjectUseCase{logger: logging.OrNop(logger), repo: repo}
}

func (uc *CreateProjectUseCase) Execute(ctx context.Context, input CreateProjectInput) (output dao.Project, err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "CreateProjectUseCase.Execute", err) }()

	projectTypeValue := input.Type
	if projectTypeValue == "" {
		projectTypeValue = "other"
	}
	projectType, err := domain.NewProjectType(projectTypeValue)
	if err != nil {
		return dao.Project{}, err
	}

	priorityValue := input.Priority
	if priorityValue == "" {
		priorityValue = "low"
	}
	priority, err := domain.NewTaskPriority(priorityValue)
	if err != nil {
		return dao.Project{}, err
	}

	project, err := domain.NewProjectWithDetails(
		input.ID,
		input.UserID,
		projectType,
		input.Title,
		input.Goal,
		input.Description,
		0,
		priority,
		input.StartDate,
		input.EndDate,
	)
	if err != nil {
		return dao.Project{}, err
	}

	return uc.repo.Create(ctx, project)

}
