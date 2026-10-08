package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type RestoreProjectUseCase struct {
	uow      UnitOfWork
	progress ProjectProgressReader
	logger   logging.Logger
}

func NewRestoreProjectUseCase(uow UnitOfWork, progress ProjectProgressReader, logger logging.Logger) *RestoreProjectUseCase {
	return &RestoreProjectUseCase{uow: uow, progress: progress, logger: logging.OrNop(logger)}
}

func (uc *RestoreProjectUseCase) Execute(ctx context.Context, actor domain.UserID, id domain.ProjectID, expected int32) (output dao.Project, err error) {
	defer func() { logUnexpectedProjectFailure(uc.logger, ctx, "RestoreProjectUseCase.Execute", err) }()
	var project dao.Project
	err = uc.uow.Do(ctx, func(ctx context.Context, repo Repository, _ ProjectChildrenDeleter) error {
		current, err := repo.LockDeletedByUserIDWithPermission(ctx, actor, id, shared.ProjectDelete())
		if err != nil {
			return err
		}
		if current.Revision != expected {
			return ErrRevisionConflict
		}
		project, err = repo.RestoreByUserID(ctx, actor, id, expected)
		if err != nil {
			return err
		}
		project.CanDelete = true
		project.CanUpdate, err = repo.HasPermission(ctx, actor, id, shared.ProjectUpdate())
		return err
	})
	if err != nil {
		return dao.Project{}, err
	}
	projects, err := applyProjectProgress(ctx, uc.progress, []dao.Project{project}, time.Now())
	if err != nil {
		return dao.Project{}, err
	}
	return projects[0], nil
}
