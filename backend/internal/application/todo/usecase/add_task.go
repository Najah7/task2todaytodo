package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/todo/dao"
	"github.com/Najah7/task2todaytodo/internal/application/todo/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type AddTaskUseCase struct {
	uow         UOW
	newID       func() string
	regenerator *RegenerateTodayUseCase
	logger      telemetry.Logger
}

func NewAddTaskUseCase(uow UOW, newID func() string, loggers ...telemetry.Logger) *AddTaskUseCase {
	var logger telemetry.Logger
	if len(loggers) > 0 {
		logger = loggers[0]
	}
	return &AddTaskUseCase{uow: uow, newID: newID, regenerator: NewRegenerateTodayUseCase(uow, newID), logger: telemetry.OrNop(logger)}
}

func (u *AddTaskUseCase) Execute(ctx context.Context, userID, taskID string, expectedRevision int32) (result dao.Todo, err error) {
	defer func() { logUnexpectedTodoFailure(u.logger, ctx, "AddTaskUseCase.Execute", err) }()
	result, err = addWork(ctx, u.uow, u.regenerator, u.newID, userID, dao.WorkSourceRef{Kind: string(domain.FrameTask), ID: taskID}, expectedRevision)
	if err == nil && result.ID != "" {
		logTodoStateChange(u.logger, ctx, "todo.task.add", userID, result.ID, taskID)
	}
	return result, err
}
