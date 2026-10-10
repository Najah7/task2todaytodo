package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

// TaskWithTags is the task read model returned by GetTaskUseCase. Child work
// items and schedules have their own list operations.
type TaskWithTags struct {
	Task dao.Task
	Tags []dao.TaskTag
}

type getTaskUOW interface {
	Do(ctx context.Context, fn func(context.Context, Repositories) error) error
}

type GetTaskUseCase struct {
	uow    getTaskUOW
	logger logging.Logger
}

func NewGetTaskUseCase(uow getTaskUOW, logger logging.Logger) *GetTaskUseCase {
	return &GetTaskUseCase{logger: logging.OrNop(logger), uow: uow}
}

func (uc *GetTaskUseCase) Execute(ctx context.Context, userID domain.UserID, taskID domain.TaskID) (output TaskWithTags, err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "GetTaskUseCase.Execute", err) }()

	var result TaskWithTags
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		taskRepo := repos.Tasks()
		task, err := taskRepo.GetByUserID(ctx, userID, taskID)
		if err != nil {
			return err
		}
		task.CanUpdate, err = taskRepo.HasPermission(ctx, userID, taskID, shared.TaskUpdate())
		if err != nil {
			return err
		}
		tasks, err := EnrichTasksInRepositories(ctx, repos, userID, []dao.Task{task}, time.Now())
		if err != nil {
			return err
		}
		task = tasks[0]

		tags, err := repos.TaskTags().ListByTaskAndUserID(ctx, userID, taskID)
		if err != nil {
			return err
		}

		result = TaskWithTags{Task: task, Tags: tags}
		return nil
	})
	if err != nil {
		return TaskWithTags{}, err
	}
	return result, nil

}
