package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type CompleteActionItemUseCase struct {
	uow    UOW
	logger telemetry.Logger
}

func NewCompleteActionItemUseCase(uow UOW, logger telemetry.Logger) *CompleteActionItemUseCase {
	return &CompleteActionItemUseCase{logger: telemetry.OrNop(logger), uow: uow}
}

func (uc *CompleteActionItemUseCase) Execute(ctx context.Context, userID domain.UserID, taskID domain.TaskID, actionItemID domain.ActionItemID) (err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "CompleteActionItemUseCase.Execute", err) }()
	asOf := time.Now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withTaskProgressMutationForPermission(ctx, repos, userID, taskID, asOf, shared.ActionItemUpdate(), func() error {
			return repos.ActionItems().CheckForOwnedTask(ctx, userID, taskID, actionItemID)
		})
	})
	if err == nil {
		logTaskStateChange(uc.logger, ctx, "action_item.complete", userID, taskID, string(actionItemID))
	}
	return err
}
