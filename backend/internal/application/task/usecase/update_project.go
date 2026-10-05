package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

var ErrProjectPatchRequiredFieldNull = errors.New("required project field cannot be null")

type updateProjectRepository interface {
	GetByUserIDWithPermission(ctx context.Context, userID domain.UserID, id domain.ProjectID, capability shared.Capability) (dao.Project, error)
	UpdateByUserID(ctx context.Context, userID domain.UserID, project domain.Project, expectedRevision int32) (dao.Project, error)
}

type UpdateProjectUseCase struct {
	repo     updateProjectRepository
	progress taskProgressSource
	logger   logging.Logger
}

func NewUpdateProjectUseCase(repo updateProjectRepository, progress taskProgressSource, logger logging.Logger) *UpdateProjectUseCase {
	return &UpdateProjectUseCase{logger: logging.OrNop(logger), repo: repo, progress: progress}
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
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "UpdateProjectUseCase.Execute", err) }()

	if (title.Present && title.Value == nil) || (projectType.Present && projectType.Value == nil) || (priority.Present && priority.Value == nil) {
		return dao.Project{}, ErrProjectPatchRequiredFieldNull
	}

	current, err := uc.repo.GetByUserIDWithPermission(ctx, userID, projectID, shared.ProjectUpdate())
	if err != nil {
		return dao.Project{}, err
	}
	project, err := projectFromDAO(current)
	if err != nil {
		logTaskRestoreFailure(uc.logger, ctx, "project.update.restore", err)
		return dao.Project{}, err
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
			return dao.Project{}, err
		}
	}
	if priority.Present {
		project.Priority, err = domain.NewTaskPriority(*priority.Value)
		if err != nil {
			return dao.Project{}, err
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
		return dao.Project{}, err
	}
	updated, err := uc.repo.UpdateByUserID(ctx, userID, validated, expectedRevision)
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
	priority, err := domain.NewTaskPriority(project.Priority.Value)
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
	return domain.NewExistingProject(
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
