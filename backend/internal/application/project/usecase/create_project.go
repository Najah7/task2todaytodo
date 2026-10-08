package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
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
	HasPermission(context.Context, domain.UserID, domain.ProjectID, shared.Capability) (bool, error)
}

type CreateProjectUseCase struct {
	repo   createProjectRepository
	logger logging.Logger
}

func NewCreateProjectUseCase(repo createProjectRepository, logger logging.Logger) *CreateProjectUseCase {
	return &CreateProjectUseCase{logger: logging.OrNop(logger), repo: repo}
}

func (uc *CreateProjectUseCase) Execute(ctx context.Context, input CreateProjectInput) (output dao.Project, err error) {
	defer func() { logUnexpectedProjectFailure(uc.logger, ctx, "CreateProjectUseCase.Execute", err) }()

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
	priority, err := domain.NewPriority(priorityValue)
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

	created, err := uc.repo.Create(ctx, project)
	if err != nil {
		return dao.Project{}, err
	}
	created.CanUpdate, err = uc.repo.HasPermission(ctx, input.UserID, domain.ProjectID(created.ID), shared.ProjectUpdate())
	if err != nil {
		return dao.Project{}, err
	}
	created.CanDelete, err = uc.repo.HasPermission(ctx, input.UserID, domain.ProjectID(created.ID), shared.ProjectDelete())
	if err != nil {
		return dao.Project{}, err
	}
	return created, nil

}
