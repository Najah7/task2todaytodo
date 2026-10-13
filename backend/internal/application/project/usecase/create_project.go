package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
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

type CreateProjectUseCase struct {
	uow    UnitOfWork
	access shared.PlanAccessReader
	logger telemetry.Logger
}

func NewCreateProjectUseCase(uow UnitOfWork, access shared.PlanAccessReader, logger telemetry.Logger) *CreateProjectUseCase {
	return &CreateProjectUseCase{logger: telemetry.OrNop(logger), uow: uow, access: access}
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

	plan, err := planAccessFor(uc.access, ctx, input.UserID)
	if err != nil {
		return dao.Project{}, err
	}
	var created dao.Project
	err = uc.uow.Do(ctx, func(ctx context.Context, repo Repository, _ ProjectChildrenDeleter) error {
		if !plan.IsPaid() {
			if err := enforceProjectCapacity(ctx, repo, input.UserID); err != nil {
				return err
			}
		}
		created, err = repo.Create(ctx, project)
		if err != nil {
			return err
		}
		created.CanUpdate, err = repo.HasPermission(ctx, input.UserID, domain.ProjectID(created.ID), shared.ProjectUpdate())
		if err != nil {
			return err
		}
		created.CanDelete, err = repo.HasPermission(ctx, input.UserID, domain.ProjectID(created.ID), shared.ProjectDelete())
		return err
	})
	return created, err

}
