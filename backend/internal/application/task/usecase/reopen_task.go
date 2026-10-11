package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type ReopenTaskUseCase struct {
	changeStatus *changeTaskStatusUseCase
	logger       telemetry.Logger
}

func NewReopenTaskUseCase(uow UOW, logger telemetry.Logger) *ReopenTaskUseCase {
	return &ReopenTaskUseCase{logger: telemetry.OrNop(logger), changeStatus: newChangeTaskStatusUseCase(uow, logger)}
}

func (uc *ReopenTaskUseCase) Execute(ctx context.Context, userID domain.UserID, taskID domain.TaskID, expectedRevision int32) (task dao.Task, err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "ReopenTaskUseCase.Execute", err) }()
	return uc.changeStatus.execute(ctx, userID, taskID, expectedRevision, taskStatusTransitionReopen)

}
