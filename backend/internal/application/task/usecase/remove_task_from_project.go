package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type removeTaskFromProjectUOW interface {
	Do(ctx context.Context, fn func(context.Context, Repositories) error) error
}

type RemoveTaskFromProjectUseCase struct {
	uow      removeTaskFromProjectUOW
	progress taskProgressSource
	access   shared.PlanAccessReader
	logger   telemetry.Logger
}

func NewRemoveTaskFromProjectUseCase(uow removeTaskFromProjectUOW, progress taskProgressSource, logger telemetry.Logger, access ...shared.PlanAccessReader) *RemoveTaskFromProjectUseCase {
	uc := &RemoveTaskFromProjectUseCase{logger: telemetry.OrNop(logger), uow: uow, progress: progress}
	if len(access) > 0 {
		uc.access = access[0]
	}
	return uc
}

func (uc *RemoveTaskFromProjectUseCase) Execute(ctx context.Context, userID domain.UserID, projectID domain.ProjectID, taskID domain.TaskID, expectedRevision int32) (output dao.Task, err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "RemoveTaskFromProjectUseCase.Execute", err) }()

	var result dao.Task
	asOf := time.Now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		project, err := repos.TaskProjects().LockProjectByUserIDWithPermission(ctx, string(userID), string(projectID), shared.TaskUpdate())
		if err != nil {
			return err
		}
		lifecycle := repos.ProjectLifecycle()
		if lifecycle == nil {
			return ErrProjectLifecycleUnavailable
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
		if task.ProjectID == string(projectID) {
			if err := requireProjectWriteAccess(ctx, repos, string(userID), string(projectID)); err != nil {
				return err
			}
		}

		// Removing a task that is already detached or belongs to another project
		// is an idempotent no-op. Keep its current priority and child records.
		if task.ProjectID != string(projectID) {
			result = task
		} else {
			if err := checkActionItemDestinationLimit(ctx, repos, uc.access, domain.UserID(task.UserID), taskID, ""); err != nil {
				return err
			}
			before, err := lifecycle.CaptureWorkState(ctx, string(projectID), asOf)
			if err != nil {
				return err
			}
			result, err = tasks.RemoveFromProjectByUserID(ctx, userID, taskID, projectID, expectedRevision)
			if err != nil {
				return err
			}
			if err := lifecycle.ReconcileWorkState(ctx, string(userID), string(projectID), before, asOf); err != nil {
				return err
			}
		}
		rows, err := EnrichTasksInRepositories(ctx, repos, userID, []dao.Task{result}, asOf)
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
