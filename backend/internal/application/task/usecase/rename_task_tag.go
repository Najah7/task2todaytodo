package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type renameTaskTagRepository interface {
	GetByUserID(ctx context.Context, userID domain.UserID, id domain.TaskTagID) (dao.TaskTag, error)
	RenameByUserID(ctx context.Context, userID domain.UserID, tag domain.TaskTag) (dao.TaskTag, error)
}

type RenameTaskTagUseCase struct {
	repo   renameTaskTagRepository
	logger logging.Logger
}

func NewRenameTaskTagUseCase(repo renameTaskTagRepository, logger logging.Logger) *RenameTaskTagUseCase {
	return &RenameTaskTagUseCase{logger: logging.OrNop(logger), repo: repo}
}

func (uc *RenameTaskTagUseCase) Execute(
	ctx context.Context,
	userID domain.UserID,
	tagID domain.TaskTagID,
	name string,
) (result dao.TaskTag, err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "RenameTaskTagUseCase.Execute", err) }()

	current, err := uc.repo.GetByUserID(ctx, userID, tagID)
	if err != nil {
		return dao.TaskTag{}, err
	}

	tag, err := domain.NewExistingTaskTag(
		domain.TaskTagID(current.ID),
		domain.UserID(current.UserID),
		current.Name,
		time.Unix(current.CreatedAt, 0),
		time.Unix(current.UpdatedAt, 0),
	)
	if err != nil {
		logTaskRestoreFailure(uc.logger, ctx, "task_tag.rename.restore", err)
		return dao.TaskTag{}, err
	}

	tag, err = tag.Rename(name)
	if err != nil {
		return dao.TaskTag{}, err
	}
	return uc.repo.RenameByUserID(ctx, userID, tag)

}
