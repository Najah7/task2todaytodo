package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type ChangeProjectStatusUseCase struct {
	uow      UnitOfWork
	progress ProjectProgressReader
	logger   logging.Logger
}

func NewChangeProjectStatusUseCase(uow UnitOfWork, progress ProjectProgressReader, logger logging.Logger) *ChangeProjectStatusUseCase {
	return &ChangeProjectStatusUseCase{uow: uow, progress: progress, logger: logging.OrNop(logger)}
}

func (uc *ChangeProjectStatusUseCase) Execute(ctx context.Context, actor domain.UserID, id domain.ProjectID, expected int32, value string) (output dao.Project, err error) {
	defer func() { logUnexpectedProjectFailure(uc.logger, ctx, "ChangeProjectStatusUseCase.Execute", err) }()
	statusValue, err := domain.NewProjectStatus(value)
	if err != nil {
		return dao.Project{}, err
	}
	var project dao.Project
	err = uc.uow.Do(ctx, func(ctx context.Context, repo Repository, _ ProjectChildrenDeleter) error {
		current, err := repo.LockByUserIDWithPermission(ctx, actor, id, shared.ProjectUpdate())
		if err != nil {
			return err
		}
		if current.Revision != expected {
			return ErrRevisionConflict
		}
		currentStatus, err := domain.NewProjectStatus(current.Status)
		if err != nil {
			return err
		}
		entity, err := projectFromDAO(current)
		if err != nil {
			return err
		}
		entity, err = entity.WithStatus(statusValue)
		if err != nil {
			return err
		}
		if currentStatus == statusValue {
			project = current
			project.CanUpdate = true
			project.CanDelete, err = repo.HasPermission(ctx, actor, id, shared.ProjectDelete())
			return err
		}
		project, err = repo.SetStatusByUserID(ctx, actor, id, entity.Status.String(), expected)
		if err != nil {
			return err
		}
		project.CanUpdate = true
		project.CanDelete, err = repo.HasPermission(ctx, actor, id, shared.ProjectDelete())
		return err
	})
	if err != nil {
		return dao.Project{}, err
	}
	uc.logger.Info(ctx, "project operation completed",
		"operation", "project.status_change",
		"user_id", string(actor),
		"project_id", string(id),
		"status", statusValue.String(),
	)
	projects, err := applyProjectProgress(ctx, uc.progress, []dao.Project{project}, time.Now())
	if err != nil {
		return dao.Project{}, err
	}
	return projects[0], nil
}
