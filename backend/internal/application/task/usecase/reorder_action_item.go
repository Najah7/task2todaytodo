package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

var ErrActionItemPositionOutOfRange = errors.New("action item position is out of range")

type ReorderActionItemUseCase struct {
	uow    UOW
	logger telemetry.Logger
}

func NewReorderActionItemUseCase(uow UOW, logger telemetry.Logger) *ReorderActionItemUseCase {
	return &ReorderActionItemUseCase{uow: uow, logger: telemetry.OrNop(logger)}
}

func (uc *ReorderActionItemUseCase) Execute(ctx context.Context, userID domain.UserID, taskID domain.TaskID, actionItemID domain.ActionItemID, position int) (output dao.ActionItem, err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "ReorderActionItemUseCase.Execute", err) }()
	if position < 0 {
		return dao.ActionItem{}, ErrActionItemPositionOutOfRange
	}
	var result dao.ActionItem
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		return withTaskProgressMutationForPermission(ctx, repos, userID, taskID, time.Now(), shared.ActionItemUpdate(), func() error {
			var err error
			result, err = repos.ActionItems().ReorderForOwnedTask(ctx, userID, taskID, actionItemID, position)
			return err
		})
	})
	return result, err
}
