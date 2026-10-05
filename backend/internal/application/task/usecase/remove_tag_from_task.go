package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type removeTagFromTaskUOW interface {
	Do(ctx context.Context, fn func(context.Context, Repositories) error) error
}

type RemoveTagFromTaskUseCase struct {
	uow    removeTagFromTaskUOW
	logger logging.Logger
}

func NewRemoveTagFromTaskUseCase(uow removeTagFromTaskUOW, logger logging.Logger) *RemoveTagFromTaskUseCase {
	return &RemoveTagFromTaskUseCase{logger: logging.OrNop(logger), uow: uow}
}

func (uc *RemoveTagFromTaskUseCase) Execute(ctx context.Context, userID domain.UserID, taskID domain.TaskID, tagID domain.TaskTagID) (err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "RemoveTagFromTaskUseCase.Execute", err) }()

	return uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		task, err := repos.Tasks().GetByUserIDWithPermission(ctx, userID, taskID, shared.TaskUpdate())
		if err != nil {
			return err
		}

		tags := repos.TaskTags()
		tagOwner := domain.UserID(task.UserID)
		tag, err := tags.GetByUserID(ctx, tagOwner, tagID)
		if err != nil {
			return err
		}
		if tag.UserID != task.UserID {
			return ErrTaskTagNotFound
		}

		// Repository removal is idempotent when this assignment does not exist.
		return tags.RemoveFromTask(ctx, userID, taskID, tagID)
	})

}
