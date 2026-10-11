package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type StartTaskUseCase struct {
	changeStatus *changeTaskStatusUseCase
	logger       telemetry.Logger
}

func NewStartTaskUseCase(uow UOW, logger telemetry.Logger) *StartTaskUseCase {
	return &StartTaskUseCase{logger: telemetry.OrNop(logger), changeStatus: newChangeTaskStatusUseCase(uow, logger)}
}

func (uc *StartTaskUseCase) Execute(ctx context.Context, userID domain.UserID, taskID domain.TaskID, expectedRevision int32) (task dao.Task, err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "StartTaskUseCase.Execute", err) }()
	return uc.changeStatus.execute(ctx, userID, taskID, expectedRevision, taskStatusTransitionStart)

}
