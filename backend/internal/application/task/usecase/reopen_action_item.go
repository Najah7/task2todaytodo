package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type ReopenActionItemUseCase struct {
	uow    UOW
	logger telemetry.Logger
}

func NewReopenActionItemUseCase(uow UOW, logger telemetry.Logger) *ReopenActionItemUseCase {
	return &ReopenActionItemUseCase{logger: telemetry.OrNop(logger), uow: uow}
}

func (uc *ReopenActionItemUseCase) Execute(ctx context.Context, userID domain.UserID, taskID domain.TaskID, actionItemID domain.ActionItemID) (err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "ReopenActionItemUseCase.Execute", err) }()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return uc.ReopenInTx(ctx, repos, userID, taskID, actionItemID, true)
	})
	if err == nil {
		logTaskStateChange(uc.logger, ctx, "action_item.reopen", userID, taskID, string(actionItemID))
	}
	return err
}

// ReopenInTx runs the Task-owned source reopen workflow in the caller's
// transaction. syncAllFrames is true for source-wide reopen and false when a
// single Todo frame is being reopened.
func (uc *ReopenActionItemUseCase) ReopenInTx(ctx context.Context, repos Repositories, userID domain.UserID, taskID domain.TaskID, actionItemID domain.ActionItemID, syncAllFrames bool) error {
	asOf := time.Now()
	return withTaskProgressMutationForPermissionAndState(ctx, repos, userID, taskID, asOf, shared.ActionItemUpdate(), func(_ dao.Task, _ taskProjectMutationSnapshot) error {
		if err := repos.ActionItems().UncheckForOwnedTask(ctx, userID, taskID, actionItemID); err != nil {
			return err
		}
		if !syncAllFrames {
			return nil
		}
		return repos.ActionItemTodos().ReopenFrames(ctx, actionItemID)
	})
}
