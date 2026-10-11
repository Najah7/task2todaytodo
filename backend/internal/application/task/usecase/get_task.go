package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type getTaskUOW interface {
	Do(ctx context.Context, fn func(context.Context, Repositories) error) error
}

type GetTaskUseCase struct {
	uow    getTaskUOW
	logger telemetry.Logger
}

func NewGetTaskUseCase(uow getTaskUOW, logger telemetry.Logger) *GetTaskUseCase {
	return &GetTaskUseCase{logger: telemetry.OrNop(logger), uow: uow}
}

func (uc *GetTaskUseCase) Execute(ctx context.Context, userID domain.UserID, taskID domain.TaskID) (output dao.Task, err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "GetTaskUseCase.Execute", err) }()

	var result dao.Task
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

		result = task
		return nil
	})
	if err != nil {
		return dao.Task{}, err
	}
	return result, nil
}
