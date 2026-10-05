package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type deleteProjectUOW interface {
	Do(ctx context.Context, fn func(context.Context, Repositories) error) error
}

type DeleteProjectUseCase struct {
	uow    deleteProjectUOW
	logger logging.Logger
}

func NewDeleteProjectUseCase(uow deleteProjectUOW, logger logging.Logger) *DeleteProjectUseCase {
	return &DeleteProjectUseCase{logger: logging.OrNop(logger), uow: uow}
}

func (uc *DeleteProjectUseCase) Execute(ctx context.Context, userID domain.UserID, projectID domain.ProjectID, expectedRevision int32) (err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "DeleteProjectUseCase.Execute", err) }()

	return uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		project, err := repos.Projects().LockByUserIDWithPermission(ctx, userID, projectID, shared.ProjectDelete())
		if err != nil {
			return err
		}
		if project.Revision != expectedRevision {
			return ErrRevisionConflict
		}
		if err := repos.Projects().LockActiveTasksForDeletion(ctx, projectID); err != nil {
			return err
		}
		return repos.Projects().DeleteByUserID(ctx, userID, projectID, expectedRevision)
	})

}
