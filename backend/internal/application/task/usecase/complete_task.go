package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type completeTaskUOW interface {
	Do(ctx context.Context, fn func(context.Context, Repositories) error) error
}

type CompleteTaskUseCase struct {
	uow    completeTaskUOW
	now    func() time.Time
	logger logging.Logger
}

func NewCompleteTaskUseCase(uow completeTaskUOW, now func() time.Time, logger logging.Logger) *CompleteTaskUseCase {
	if now == nil {
		now = time.Now
	}
	return &CompleteTaskUseCase{logger: logging.OrNop(logger), uow: uow, now: now}
}

func (uc *CompleteTaskUseCase) Execute(ctx context.Context, userID domain.UserID, taskID domain.TaskID, expectedRevision int32) (changed dao.Task, err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "CompleteTaskUseCase.Execute", err) }()

	asOf := uc.now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		tasks := repos.Tasks()
		current, projectBefore, err := lockTaskAndCaptureProjectStateUsing(ctx, repos, tasks, userID, taskID, asOf, shared.TaskUpdate())
		if err != nil {
			return err
		}
		if current.Revision != expectedRevision {
			return ErrRevisionConflict
		}
		task, err := taskFromDAO(current)
		if err != nil {
			logTaskRestoreFailure(uc.logger, ctx, "task.complete.restore", err)
			return err
		}

		completed := task.Complete()
		if err := tasks.SetStatusByUserIDWithPermission(ctx, userID, taskID, completed.Status, expectedRevision, shared.TaskUpdate()); err != nil {
			return err
		}
		changed, err = tasks.LockByUserIDWithPermission(ctx, userID, taskID, shared.TaskUpdate())
		if err != nil {
			return err
		}
		if projectBefore.ID != "" {
			return repos.ProjectLifecycle().ReconcileWorkState(ctx, string(userID), projectBefore.ID, projectBefore.State, asOf)
		}
		return nil
	})
	if err == nil {
		logTaskStateChange(uc.logger, ctx, "task.complete", userID, taskID, "")
	}
	return changed, err
}
