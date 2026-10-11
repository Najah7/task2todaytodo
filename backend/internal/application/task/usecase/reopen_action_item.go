package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type ReopenActionItemUseCase struct {
	uow    UOW
	logger logging.Logger
}

func NewReopenActionItemUseCase(uow UOW, logger logging.Logger) *ReopenActionItemUseCase {
	return &ReopenActionItemUseCase{logger: logging.OrNop(logger), uow: uow}
}

func (uc *ReopenActionItemUseCase) Execute(ctx context.Context, userID domain.UserID, taskID domain.TaskID, actionItemID domain.ActionItemID) (err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "ReopenActionItemUseCase.Execute", err) }()
	asOf := time.Now()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withTaskProgressMutationForPermission(ctx, repos, userID, taskID, asOf, shared.ActionItemUpdate(), func() error {
			return repos.ActionItems().UncheckForOwnedTask(ctx, userID, taskID, actionItemID)
		})
	})
	if err == nil {
		logTaskStateChange(uc.logger, ctx, "action_item.reopen", userID, taskID, string(actionItemID))
	}
	return err
}
