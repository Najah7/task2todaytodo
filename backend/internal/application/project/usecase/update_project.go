package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

var ErrProjectPatchRequiredFieldNull = errors.New("required project field cannot be null")

type updateProjectRepository interface {
	LockByUserIDWithPermission(ctx context.Context, userID domain.UserID, id domain.ProjectID, capability shared.Capability) (dao.Project, error)
	UpdateByUserID(ctx context.Context, userID domain.UserID, project domain.Project, expectedRevision int32) (dao.Project, error)
	HasPermission(context.Context, domain.UserID, domain.ProjectID, shared.Capability) (bool, error)
}

type UpdateProjectUseCase struct {
	repo     updateProjectRepository
	progress ProjectProgressReader
	logger   telemetry.Logger
	uow      UnitOfWork
}

func NewUpdateProjectUseCase(repo updateProjectRepository, progress ProjectProgressReader, logger telemetry.Logger, uows ...UnitOfWork) *UpdateProjectUseCase {
	var uow UnitOfWork
	if len(uows) > 0 {
		uow = uows[0]
	}
	return &UpdateProjectUseCase{logger: telemetry.OrNop(logger), repo: repo, progress: progress, uow: uow}
}

func (uc *UpdateProjectUseCase) Execute(
	ctx context.Context,
	userID domain.UserID,
	projectID domain.ProjectID,
	expectedRevision int32,
	title PatchField[string],
	goal PatchField[string],
	description PatchField[string],
	projectType PatchField[string],
	priority PatchField[string],
	startDate PatchField[time.Time],
	endDate PatchField[time.Time],
) (result dao.Project, err error) {
	defer func() { logUnexpectedProjectFailure(uc.logger, ctx, "UpdateProjectUseCase.Execute", err) }()

	if (title.Present && title.Value == nil) || (projectType.Present && projectType.Value == nil) || (priority.Present && priority.Value == nil) {
		return dao.Project{}, ErrProjectPatchRequiredFieldNull
	}

	var updated dao.Project
	operation := func(ctx context.Context, repo updateProjectRepository) error {
		current, err := repo.LockByUserIDWithPermission(ctx, userID, projectID, shared.ProjectUpdate())
		if err != nil {
			return err
		}
		if err := requireProjectWriter(ctx, repo, userID, projectID); err != nil {
			return err
		}
		project, err := projectFromDAO(current)
		if err != nil {
			logProjectRestoreFailure(uc.logger, ctx, "project.update.restore", err)
			return err
		}

		if title.Present {
			project.Title = *title.Value
		}
		if goal.Present {
			project.Goal = ""
			if goal.Value != nil {
				project.Goal = *goal.Value
			}
		}
		if description.Present {
			project.Description = ""
			if description.Value != nil {
				project.Description = *description.Value
			}
		}
		if projectType.Present {
			project.Type, err = domain.NewProjectType(*projectType.Value)
			if err != nil {
				return err
			}
		}
		if priority.Present {
			project.Priority, err = domain.NewPriority(*priority.Value)
			if err != nil {
				return err
			}
		}
		if startDate.Present {
			project.Schedule.StartDate = nil
			if startDate.Value != nil {
				date := *startDate.Value
				project.Schedule.StartDate = &date
			}
		}
		if endDate.Present {
			project.Schedule.EndDate = nil
			if endDate.Value != nil {
				date := *endDate.Value
				project.Schedule.EndDate = &date
			}
		}

		validated, err := domain.NewExistingProject(
			project.ID,
			project.UserID,
			project.Type,
			project.Title,
			project.Goal,
			project.Description,
			project.Progress,
			project.Priority,
			project.Schedule.StartDate,
			project.Schedule.EndDate,
			project.CreatedAt,
			project.UpdatedAt,
		)
		if err != nil {
			return err
		}
		updated, err = repo.UpdateByUserID(ctx, userID, validated, expectedRevision)
		if err != nil {
			return err
		}
		return readProjectCapabilities(ctx, repo, userID, &updated)
	}
	if uc.uow != nil {
		err = uc.uow.Do(ctx, func(ctx context.Context, repo Repository, _ ProjectChildrenDeleter) error {
			return operation(ctx, repo)
		})
	} else {
		err = operation(ctx, uc.repo)
	}
	if err != nil {
		return dao.Project{}, err
	}
	projects, err := applyProjectProgress(ctx, uc.progress, []dao.Project{updated}, time.Now())
	if err != nil {
		return dao.Project{}, err
	}
	return projects[0], nil

}

func projectFromDAO(project dao.Project) (domain.Project, error) {
	projectType, err := domain.NewProjectType(project.Type.Value)
	if err != nil {
		return domain.Project{}, err
	}
	priority, err := domain.NewPriority(project.Priority.Value)
	if err != nil {
		return domain.Project{}, err
	}
	startDate, err := parseProjectDate(project.StartDate)
	if err != nil {
		return domain.Project{}, fmt.Errorf("restore project start date: %w", err)
	}
	endDate, err := parseProjectDate(project.EndDate)
	if err != nil {
		return domain.Project{}, fmt.Errorf("restore project end date: %w", err)
	}
	projectEntity, err := domain.NewExistingProject(
		domain.ProjectID(project.ID),
		domain.UserID(project.UserID),
		projectType,
		project.Title,
		project.Goal,
		project.Description,
		project.Progress,
		priority,
		startDate,
		endDate,
		time.Unix(project.CreatedAt, 0),
		time.Unix(project.UpdatedAt, 0),
	)
	if err != nil {
		return domain.Project{}, err
	}
	status, err := domain.NewProjectStatus(project.Status)
	if err != nil {
		return domain.Project{}, err
	}
	return projectEntity.WithStatus(status)
}

func parseProjectDate(value *string) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	date, err := time.Parse("2006-01-02", *value)
	if err != nil {
		return nil, err
	}
	return &date, nil
}
