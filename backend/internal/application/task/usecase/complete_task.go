package usecase

import (
	"context"
	"math"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type completeTaskUOW interface {
	Do(ctx context.Context, fn func(context.Context, Repositories) error) error
}

type CompleteTaskUseCase struct {
	uow    completeTaskUOW
	now    func() time.Time
	logger telemetry.Logger
}

func NewCompleteTaskUseCase(uow completeTaskUOW, now func() time.Time, logger telemetry.Logger) *CompleteTaskUseCase {
	if now == nil {
		now = time.Now
	}
	return &CompleteTaskUseCase{logger: telemetry.OrNop(logger), uow: uow, now: now}
}

func (uc *CompleteTaskUseCase) Execute(ctx context.Context, userID domain.UserID, taskID domain.TaskID, expectedRevision int32) (changed dao.Task, err error) {
	return uc.ExecuteWithActual(ctx, userID, taskID, expectedRevision, nil)
}

func (uc *CompleteTaskUseCase) ExecuteWithActual(ctx context.Context, userID domain.UserID, taskID domain.TaskID, expectedRevision int32, actualMinutes *int) (changed dao.Task, err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "CompleteTaskUseCase.Execute", err) }()
	if actualMinutes != nil && (*actualMinutes < 0 || int64(*actualMinutes) > math.MaxInt32) {
		return dao.Task{}, domain.ErrTaskActualMinutesInvalid
	}
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		var inner error
		changed, inner = uc.CompleteInTx(ctx, repos, userID, taskID, expectedRevision, actualMinutes, true)
		return inner
	})
	if err == nil {
		logTaskStateChange(uc.logger, ctx, "task.complete", userID, taskID, "")
	}
	return changed, err
}

// CompleteInTx is the Task-owned completion workflow. syncTodayFrames is true
// for the Tasks/Inbox source command and false for a Todo frame which already
// owns its selected-frame mutation.
func (uc *CompleteTaskUseCase) CompleteInTx(ctx context.Context, repos Repositories, userID domain.UserID, taskID domain.TaskID, expectedRevision int32, actualMinutes *int, syncTodayFrames bool) (dao.Task, error) {
	asOf := uc.now()
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
	completed := task.Complete()
	completed.ActualMinutes = copyOptionalInt(actualMinutes)
	validated, err := domain.NewExistingTask(completed.ID, completed.UserID, completed.ProjectID, completed.Title, completed.Description, completed.DueDate, completed.ManualEstimatedMinutes, completed.ActualMinutes, completed.Progress, completed.Priority, completed.Status, completed.AssigneeID, completed.CreatedAt, completed.UpdatedAt)
	if err != nil {
		return dao.Task{}, err
	}
	changed, err := tasks.UpdateByUserID(ctx, userID, validated, expectedRevision)
	if err != nil {
		return dao.Task{}, err
	}
	if syncTodayFrames {
		assignee := domain.UserID(current.AssigneeID)
		if assignee == "" {
			assignee = userID
		}
		if err = repos.TaskTodos().CompleteTodayTaskFrames(ctx, assignee, taskID); err != nil {
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

// CompleteFromTodoFrame is the Todo consumer command and deliberately does not
// sync frames back: Todo already completed the triggering frame.
func (uc *CompleteTaskUseCase) CompleteFromTodoFrame(ctx context.Context, repos Repositories, actorID, taskID string, actualMinutes *int) error {
	current, err := repos.Tasks().LockByUserIDWithPermission(ctx, domain.UserID(actorID), domain.TaskID(taskID), shared.TaskUpdate())
	if err != nil {
		return err
	}
	_, err = uc.CompleteInTx(ctx, repos, domain.UserID(actorID), domain.TaskID(taskID), current.Revision, actualMinutes, false)
	return err
}
