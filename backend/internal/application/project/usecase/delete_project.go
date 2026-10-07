package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type DeleteProjectUseCase struct {
	uow    DeleteProjectUnitOfWork
	logger logging.Logger
}

func NewDeleteProjectUseCase(uow DeleteProjectUnitOfWork, logger logging.Logger) *DeleteProjectUseCase {
	return &DeleteProjectUseCase{uow: uow, logger: logging.OrNop(logger)}
}
func (uc *DeleteProjectUseCase) Execute(ctx context.Context, actor domain.UserID, projectID domain.ProjectID, expectedRevision int32) (err error) {
	defer func() { logUnexpectedProjectFailure(uc.logger, ctx, "DeleteProjectUseCase.Execute", err) }()
	return uc.uow.Do(ctx, func(ctx context.Context, repo Repository, children ProjectChildrenDeleter) error {
		project, err := repo.LockByUserIDWithPermission(ctx, actor, projectID, shared.ProjectDelete())
		if err != nil {
			return err
		}
		if project.Revision != expectedRevision {
			return ErrRevisionConflict
		}
		if err := children.DeleteProjectTasks(ctx, string(actor), string(projectID)); err != nil {
			return err
		}
		if err := children.DeleteProjectSchedules(ctx, string(actor), string(projectID)); err != nil {
			return err
		}
		return repo.DeleteByUserID(ctx, actor, projectID, expectedRevision)
	})
}
