package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type removeTaskFromProjectUOW interface {
	Do(ctx context.Context, fn func(context.Context, Repositories) error) error
}

type RemoveTaskFromProjectUseCase struct {
	uow      removeTaskFromProjectUOW
	progress taskProgressSource
	logger   logging.Logger
}

func NewRemoveTaskFromProjectUseCase(uow removeTaskFromProjectUOW, progress taskProgressSource, logger logging.Logger) *RemoveTaskFromProjectUseCase {
	return &RemoveTaskFromProjectUseCase{logger: logging.OrNop(logger), uow: uow, progress: progress}
}

func (uc *RemoveTaskFromProjectUseCase) Execute(ctx context.Context, userID domain.UserID, projectID domain.ProjectID, taskID domain.TaskID, expectedRevision int32) (output dao.Task, err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "RemoveTaskFromProjectUseCase.Execute", err) }()

	var result dao.Task
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		project, err := repos.TaskProjects().LockProjectByUserIDWithPermission(ctx, string(userID), string(projectID), shared.TaskUpdate())
		if err != nil {
			return err
		}

		tasks := repos.Tasks()
		task, err := tasks.GetByUserIDWithPermission(ctx, userID, taskID, shared.TaskUpdate())
		if err != nil {
			return err
		}
		if task.UserID != project.OwnerID {
			return ErrTaskNotFound
		}
		if task.Revision != expectedRevision {
			return ErrRevisionConflict
		}

		// Removing a task that is already detached or belongs to another project
		// is an idempotent no-op. Keep its current priority and child records.
		if task.ProjectID != string(projectID) {
			result = task
		} else {
			result, err = tasks.RemoveFromProjectByUserID(ctx, userID, taskID, projectID, expectedRevision)
			if err != nil {
				return err
			}
		}
		rows, err := applyTaskProgress(ctx, tasks, []dao.Task{result}, time.Now())
		if err != nil {
			return err
		}
		result = rows[0]
		return nil
	})
	if err != nil {
		return dao.Task{}, err
	}
	return result, nil

}
