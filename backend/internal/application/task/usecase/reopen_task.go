package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type ReopenTaskUseCase struct {
	uow    UOW
	logger telemetry.Logger
}

func NewReopenTaskUseCase(uow UOW, logger telemetry.Logger) *ReopenTaskUseCase {
	return &ReopenTaskUseCase{logger: telemetry.OrNop(logger), uow: uow}
}

func (uc *ReopenTaskUseCase) Execute(ctx context.Context, userID domain.UserID, taskID domain.TaskID, expectedRevision int32) (task dao.Task, err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "ReopenTaskUseCase.Execute", err) }()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		var inner error
		task, inner = uc.ReopenInTx(ctx, repos, userID, taskID, expectedRevision, true)
		return inner
	})
	if err == nil {
		logTaskStateChange(uc.logger, ctx, "task.reopen", userID, taskID, "")
	}
	return task, err
}

// ReopenInTx reopens a Task and optionally clears all historical Task frames.
// Todo calls it with syncAllFrames=false for its selected frame transition.
func (uc *ReopenTaskUseCase) ReopenInTx(ctx context.Context, repos Repositories, userID domain.UserID, taskID domain.TaskID, expectedRevision int32, syncAllFrames bool) (dao.Task, error) {
	asOf := time.Now()
	tasks := repos.Tasks()
	current, projectBefore, err := lockTaskAndCaptureProjectStateUsing(ctx, repos, tasks, userID, taskID, asOf, shared.TaskUpdate())
	if err != nil {
		return dao.Task{}, err
	}
	if current.Revision != expectedRevision {
		return dao.Task{}, ErrRevisionConflict
	}
	task, err := taskFromDAO(current)
	if err != nil {
		return dao.Task{}, err
	}
	reopened := task.Reopen()
	reopened.ActualMinutes = nil
	validated, err := domain.NewExistingTask(reopened.ID, reopened.UserID, reopened.ProjectID, reopened.Title, reopened.Description, reopened.DueDate, reopened.ManualEstimatedMinutes, nil, reopened.Progress, reopened.Priority, reopened.Status, reopened.AssigneeID, reopened.CreatedAt, reopened.UpdatedAt)
	if err != nil {
		return dao.Task{}, err
	}
	changed, err := tasks.UpdateByUserID(ctx, userID, validated, expectedRevision)
	if err != nil {
		return dao.Task{}, err
	}
	if syncAllFrames {
		if err = repos.TaskTodos().ReopenTaskFrames(ctx, taskID); err != nil {
			return dao.Task{}, err
		}
	}
	if projectBefore.ID != "" {
		if err = repos.ProjectLifecycle().ReconcileWorkState(ctx, string(userID), projectBefore.ID, projectBefore.State, asOf); err != nil {
			return dao.Task{}, err
		}
	}
	return changed, nil
}

// ReopenFromTodoFrame is the Todo consumer command. It resets only source
// status/actual; the caller has already reopened the selected frame.
func (uc *ReopenTaskUseCase) ReopenFromTodoFrame(ctx context.Context, repos Repositories, actorID, taskID string) error {
	current, err := repos.Tasks().LockByUserIDWithPermission(ctx, domain.UserID(actorID), domain.TaskID(taskID), shared.TaskUpdate())
	if err != nil {
		return err
	}
	_, err = uc.ReopenInTx(ctx, repos, domain.UserID(actorID), domain.TaskID(taskID), current.Revision, false)
	return err
}
