package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type DeleteProjectUseCase struct {
	uow    DeleteProjectUnitOfWork
	logger telemetry.Logger
}

func NewDeleteProjectUseCase(uow DeleteProjectUnitOfWork, logger telemetry.Logger) *DeleteProjectUseCase {
	return &DeleteProjectUseCase{uow: uow, logger: telemetry.OrNop(logger)}
}
func (uc *DeleteProjectUseCase) Execute(ctx context.Context, actor domain.UserID, projectID domain.ProjectID, expectedRevision int32) (err error) {
	defer func() { logUnexpectedProjectFailure(uc.logger, ctx, "DeleteProjectUseCase.Execute", err) }()
	return uc.uow.Do(ctx, func(ctx context.Context, repo Repository, _ ProjectChildrenDeleter) error {
		project, err := repo.LockByUserIDWithPermission(ctx, actor, projectID, shared.ProjectDelete())
		if err != nil {
			return err
		}
		if err := requireProjectWriter(ctx, repo, actor, projectID); err != nil {
			return err
		}
		if project.DeletedAt == nil {
			if err := repo.LockUserForPlanLimit(ctx, domain.UserID(project.UserID)); err != nil {
				return err
			}
		}
		if project.Revision != expectedRevision {
			return ErrRevisionConflict
		}
		return repo.DeleteByUserID(ctx, actor, projectID, expectedRevision)
	})
}
