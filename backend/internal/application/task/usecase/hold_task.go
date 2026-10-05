package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type HoldTaskUseCase struct {
	changeStatus *changeTaskStatusUseCase
	logger       logging.Logger
}

func NewHoldTaskUseCase(uow UOW, logger logging.Logger) *HoldTaskUseCase {
	return &HoldTaskUseCase{logger: logging.OrNop(logger), changeStatus: newChangeTaskStatusUseCase(uow, logger)}
}

func (uc *HoldTaskUseCase) Execute(ctx context.Context, userID domain.UserID, taskID domain.TaskID, expectedRevision int32) (task dao.Task, err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "HoldTaskUseCase.Execute", err) }()
	return uc.changeStatus.execute(ctx, userID, taskID, expectedRevision, taskStatusTransitionHold)

}
