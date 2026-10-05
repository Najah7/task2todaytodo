package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type AssignTaskUseCase struct {
	uow    UOW
	logger logging.Logger
}

func NewAssignTaskUseCase(uow UOW, logger logging.Logger) *AssignTaskUseCase {
	return &AssignTaskUseCase{uow: uow, logger: logging.OrNop(logger)}
}

func (uc *AssignTaskUseCase) Execute(
	ctx context.Context,
	actorID domain.UserID,
	taskID domain.TaskID,
	assigneeID domain.UserID,
	expectedRevision int32,
) (result dao.Task, err error) {
	defer func() {
		if errors.Is(err, ErrPermissionDenied) || errors.Is(err, ErrRevisionConflict) || errors.Is(err, domain.ErrTaskAssigneeEmpty) {
			return
		}
		logUnexpectedTaskFailure(uc.logger, ctx, "AssignTaskUseCase.Execute", err)
	}()

	if strings.TrimSpace(string(actorID)) == "" {
		return dao.Task{}, domain.ErrTaskUserIDEmpty
	}
	if strings.TrimSpace(string(taskID)) == "" {
		return dao.Task{}, domain.ErrTaskIDEmpty
	}
	if strings.TrimSpace(string(assigneeID)) == "" {
		return dao.Task{}, domain.ErrTaskAssigneeEmpty
	}
	if expectedRevision <= 0 {
		return dao.Task{}, ErrRevisionConflict
	}

	var updatedTask dao.Task
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		repository := repos.Tasks()

		// Assignment and project_member removal acquire the same project lock. Read
		// eligibility only after acquiring it so this statement sees any removal
		// that committed while assignment waited for the lock.
		if err := repository.LockProjectForTaskAssignment(ctx, actorID, taskID); err != nil {
			return err
		}
		eligible, err := repository.IsEligibleTaskAssignee(ctx, actorID, taskID, assigneeID)
		if err != nil {
			return err
		}
		if !eligible {
			return ErrPermissionDenied
		}
		current, err := repository.Get(ctx, taskID)
		if err != nil {
			return err
		}
		task, err := taskFromDAO(current)
		if err != nil {
			logTaskRestoreFailure(uc.logger, ctx, "task.assign.restore", err)
			return err
		}
		updated, err := task.AssignTo(assigneeID)
		if err != nil {
			return err
		}
		updatedTask, err = repository.UpdateTaskAssigneeByActor(ctx, actorID, updated.ID, updated.AssigneeID, expectedRevision)
		if err != nil {
			return err
		}
		rows, err := applyTaskProgress(ctx, repository, []dao.Task{updatedTask}, time.Now())
		if err != nil {
			return err
		}
		updatedTask = rows[0]
		return nil
	})
	if err != nil {
		return dao.Task{}, err
	}
	logTaskStateChange(uc.logger, ctx, "task.assign", actorID, taskID, "")
	return updatedTask, nil
}
