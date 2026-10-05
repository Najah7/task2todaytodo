package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type deleteTaskTagRepository interface {
	DeleteByUserID(ctx context.Context, userID domain.UserID, tagID domain.TaskTagID) error
}

type DeleteTaskTagUseCase struct {
	repo   deleteTaskTagRepository
	logger logging.Logger
}

func NewDeleteTaskTagUseCase(repo deleteTaskTagRepository, logger logging.Logger) *DeleteTaskTagUseCase {
	return &DeleteTaskTagUseCase{logger: logging.OrNop(logger), repo: repo}
}

func (uc *DeleteTaskTagUseCase) Execute(ctx context.Context, userID domain.UserID, tagID domain.TaskTagID) (err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "DeleteTaskTagUseCase.Execute", err) }()

	return uc.repo.DeleteByUserID(ctx, userID, tagID)

}
