package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

var ErrTaskPatchRequiredFieldNull = errors.New("required task field cannot be null")

type updateTaskRepository interface {
	GetByUserIDWithPermission(ctx context.Context, userID domain.UserID, id domain.TaskID, capability shared.Capability) (dao.Task, error)
	UpdateByUserID(ctx context.Context, userID domain.UserID, task domain.Task, expectedRevision int32) (dao.Task, error)
}

type UpdateTaskUseCase struct {
	repo     updateTaskRepository
	progress taskProgressSource
	logger   logging.Logger
}

func NewUpdateTaskUseCase(repo updateTaskRepository, progress taskProgressSource, logger logging.Logger) *UpdateTaskUseCase {
	return &UpdateTaskUseCase{logger: logging.OrNop(logger), repo: repo, progress: progress}
}

func (uc *UpdateTaskUseCase) Execute(
	ctx context.Context,
	userID domain.UserID,
	taskID domain.TaskID,
	expectedRevision int32,
	title PatchField[string],
	description PatchField[string],
	dueDate PatchField[time.Time],
	estimatedMinutes PatchField[int],
	actualMinutes PatchField[int],
) (result dao.Task, err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "UpdateTaskUseCase.Execute", err) }()

	if title.Present && title.Value == nil {
		return dao.Task{}, ErrTaskPatchRequiredFieldNull
	}

	current, err := uc.repo.GetByUserIDWithPermission(ctx, userID, taskID, shared.TaskUpdate())
	if err != nil {
		return dao.Task{}, err
	}
	task, err := taskFromDAO(current)
	if err != nil {
		logTaskRestoreFailure(uc.logger, ctx, "task.update.restore", err)
		return dao.Task{}, err
	}

	if title.Present {
		task.Title = *title.Value
	}
	if description.Present {
		task.Description = ""
		if description.Value != nil {
			task.Description = *description.Value
		}
	}
	if dueDate.Present {
		task.DueDate = time.Time{}
		if dueDate.Value != nil {
			task.DueDate = *dueDate.Value
		}
	}
	if estimatedMinutes.Present {
		task.EstimatedMinutes = copyOptionalInt(estimatedMinutes.Value)
	}
	if actualMinutes.Present {
		task.ActualMinutes = copyOptionalInt(actualMinutes.Value)
	}

	validated, err := domain.NewExistingTask(
		task.ID,
		task.UserID,
		task.ProjectID,
		task.Title,
		task.Description,
		task.DueDate,
		task.EstimatedMinutes,
		task.ActualMinutes,
		task.Progress,
		task.Priority,
		task.Status,
		task.AssigneeID,
		task.CreatedAt,
		task.UpdatedAt,
	)
	if err != nil {
		return dao.Task{}, err
	}
	updated, err := uc.repo.UpdateByUserID(ctx, userID, validated, expectedRevision)
	if err != nil {
		return dao.Task{}, err
	}
	tasks, err := applyTaskProgress(ctx, uc.progress, []dao.Task{updated}, time.Now())
	if err != nil {
		return dao.Task{}, err
	}
	return tasks[0], nil

}

func taskFromDAO(task dao.Task) (domain.Task, error) {
	priority, err := domain.NewTaskPriority(task.Priority.Value)
	if err != nil {
		return domain.Task{}, err
	}
	status, err := domain.NewTaskStatus(task.Status.Value)
	if err != nil {
		return domain.Task{}, err
	}
	var dueDate time.Time
	if task.DueDate != 0 {
		dueDate = time.Unix(task.DueDate, 0)
	}
	result, err := domain.NewExistingTask(
		domain.TaskID(task.ID),
		domain.UserID(task.UserID),
		domain.ProjectID(task.ProjectID),
		task.Title,
		task.Description,
		dueDate,
		copyOptionalInt(task.EstimatedMinutes),
		copyOptionalInt(task.ActualMinutes),
		task.Progress,
		priority,
		status,
		domain.UserID(task.AssigneeID),
		time.Unix(task.CreatedAt, 0),
		time.Unix(task.UpdatedAt, 0),
	)
	return result, err
}

func copyOptionalInt(value *int) *int {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
