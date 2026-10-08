package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type addTagToTaskUOW interface {
	Do(ctx context.Context, fn func(context.Context, Repositories) error) error
}

type AddTagToTaskUseCase struct {
	uow    addTagToTaskUOW
	logger logging.Logger
}

func NewAddTagToTaskUseCase(uow addTagToTaskUOW, logger logging.Logger) *AddTagToTaskUseCase {
	return &AddTagToTaskUseCase{logger: logging.OrNop(logger), uow: uow}
}

func (uc *AddTagToTaskUseCase) Execute(ctx context.Context, userID domain.UserID, taskID domain.TaskID, tagID string) (err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "AddTagToTaskUseCase.Execute", err) }()

	return uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		if _, err := lockTaskForMutation(ctx, repos, userID, taskID, shared.TaskUpdate()); err != nil {
			return err
		}

		return repos.TaskTags().AddToTask(ctx, userID, taskID, tagID)
	})

}
