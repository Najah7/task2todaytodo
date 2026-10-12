package usecase

import (
	"context"
	"math"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
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

func (uc *CompleteActionItemUseCase) Execute(ctx context.Context, userID domain.UserID, taskID domain.TaskID, actionItemID domain.ActionItemID, actualMinutes ...*int) (err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "CompleteActionItemUseCase.Execute", err) }()
	err = uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		var actual *int
		if len(actualMinutes) > 0 {
			actual = actualMinutes[0]
		}
		return uc.CompleteInTx(ctx, repos, userID, taskID, actionItemID, actual, true)
	})
	if err == nil {
		logTaskStateChange(uc.logger, ctx, "action_item.complete", userID, taskID, string(actionItemID))
	}
	return err
}

// CompleteInTx runs the Task-owned ActionItem completion and Task/Project
// lifecycle workflow in the caller's transaction. syncTodayFrames is true for
// Tasks/Inbox source completion and false for a Todo frame that already owns
// its selected-frame update.
func (uc *CompleteActionItemUseCase) CompleteInTx(ctx context.Context, repos Repositories, userID domain.UserID, taskID domain.TaskID, actionItemID domain.ActionItemID, actualMinutes *int, syncTodayFrames bool) error {
	if actualMinutes != nil && (*actualMinutes < 0 || int64(*actualMinutes) > math.MaxInt32) {
		return domain.ErrActionItemActualMinutesInvalid
	}
	asOf := time.Now()
	return withTaskProgressMutationForPermissionAndState(ctx, repos, userID, taskID, asOf, shared.ActionItemUpdate(), func(task dao.Task, _ taskProjectMutationSnapshot) error {
		var err error
		if actualMinutes == nil {
			err = repos.ActionItems().CheckForOwnedTask(ctx, userID, taskID, actionItemID)
		} else {
			err = repos.ActionItems().CompleteWithActualMinutesForOwnedTask(ctx, userID, taskID, actionItemID, actualMinutes)
		}
		if err != nil {
			return err
		}
		if !syncTodayFrames {
			return nil
		}
		assigneeID := domain.UserID(task.AssigneeID)
		if assigneeID == "" {
			assigneeID = userID
		}
		return repos.ActionItemTodos().CompleteTodayFrames(ctx, assigneeID, actionItemID)
	})
}
