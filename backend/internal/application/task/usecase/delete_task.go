package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type deleteTaskUOW interface {
	Do(ctx context.Context, fn func(context.Context, Repositories) error) error
}

type DeleteTaskUseCase struct {
	uow    deleteTaskUOW
	logger logging.Logger
}

func NewDeleteTaskUseCase(uow deleteTaskUOW, logger logging.Logger) *DeleteTaskUseCase {
	return &DeleteTaskUseCase{logger: logging.OrNop(logger), uow: uow}
}

func (uc *DeleteTaskUseCase) Execute(ctx context.Context, userID domain.UserID, taskID domain.TaskID, expectedRevision int32) (err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "DeleteTaskUseCase.Execute", err) }()

	asOf := time.Now()
	return uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		current, projectBefore, err := lockTaskAndCaptureProjectState(ctx, repos, userID, taskID, asOf, shared.TaskDelete())
		if err != nil {
			return err
		}
		if current.Revision != expectedRevision {
			return ErrRevisionConflict
		}
		if err := repos.Tasks().DeleteByUserID(ctx, userID, taskID, expectedRevision); err != nil {
			return err
		}
		if projectBefore.ID != "" {
			return repos.ProjectLifecycle().ReconcileWorkState(ctx, string(userID), projectBefore.ID, projectBefore.State, asOf)
		}
		return nil
	})

}
