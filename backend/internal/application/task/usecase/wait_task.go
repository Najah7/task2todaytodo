package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type WaitTaskUseCase struct {
	changeStatus *changeTaskStatusUseCase
	logger       telemetry.Logger
}

func NewWaitTaskUseCase(uow UOW, logger telemetry.Logger) *WaitTaskUseCase {
	return &WaitTaskUseCase{logger: telemetry.OrNop(logger), changeStatus: newChangeTaskStatusUseCase(uow, logger)}
}

func (uc *WaitTaskUseCase) Execute(ctx context.Context, userID domain.UserID, taskID domain.TaskID, expectedRevision int32) (task dao.Task, err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "WaitTaskUseCase.Execute", err) }()
	return uc.changeStatus.execute(ctx, userID, taskID, expectedRevision, taskStatusTransitionWait)

}
