package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type taskStatusTransition uint8

const (
	taskStatusTransitionStart taskStatusTransition = iota + 1
	taskStatusTransitionHold
	taskStatusTransitionWait
	taskStatusTransitionReopen
)

type changeTaskStatusUseCase struct {
	uow    UOW
	logger logging.Logger
}

func newChangeTaskStatusUseCase(uow UOW, logger logging.Logger) *changeTaskStatusUseCase {
	return &changeTaskStatusUseCase{uow: uow, logger: logging.OrNop(logger)}
}

func (uc *changeTaskStatusUseCase) execute(
	ctx context.Context,
	userID domain.UserID,
	taskID domain.TaskID,
	expectedRevision int32,
	transition taskStatusTransition,
) (dao.Task, error) {
	var changed dao.Task
	asOf := time.Now()
	err := uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		tasks := repos.Tasks()
		permission := shared.TaskUpdate()
		current, projectBefore, err := lockTaskAndCaptureProjectStateUsing(ctx, repos, tasks, userID, taskID, asOf, permission)
		if err != nil {
			return err
		}
		if current.ID != string(taskID) {
			return ErrTaskNotFound
		}
		if current.Revision != expectedRevision {
			return ErrRevisionConflict
		}
		task, err := taskFromDAO(current)
		if err != nil {
			logTaskRestoreFailure(uc.logger, ctx, "task_status.restore_task", err)
			return err
		}

		var updated domain.Task
		switch transition {
		case taskStatusTransitionStart:
			updated = task.Start()
		case taskStatusTransitionHold:
			updated = task.Hold()
		case taskStatusTransitionWait:
			updated = task.Wait()
		case taskStatusTransitionReopen:
			updated = task.Reopen()
		default:
			return fmt.Errorf("unsupported task status transition %d", transition)
		}
		if err := tasks.SetStatusByUserIDWithPermission(ctx, userID, taskID, updated.Status, expectedRevision, permission); err != nil {
			return err
		}
		changed, err = tasks.LockByUserIDWithPermission(ctx, userID, taskID, permission)
		if err != nil {
			return err
		}
		if projectBefore.ID != "" {
			return repos.ProjectLifecycle().ReconcileWorkState(ctx, string(userID), projectBefore.ID, projectBefore.State, asOf)
		}
		return nil
	})
	if err != nil {
		return dao.Task{}, err
	}
	uc.logger.Info(ctx, "task operation completed",
		"operation", taskStatusOperation(transition),
		"user_id", string(userID),
		"task_id", string(taskID),
	)
	return changed, nil
}

func taskStatusOperation(transition taskStatusTransition) string {
	switch transition {
	case taskStatusTransitionStart:
		return "task.start"
	case taskStatusTransitionHold:
		return "task.hold"
	case taskStatusTransitionWait:
		return "task.wait"
	case taskStatusTransitionReopen:
		return "task.reopen"
	default:
		return "task.status_change"
	}
}
