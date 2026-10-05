package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type CreateTaskTagInput struct {
	ID     domain.TaskTagID
	UserID domain.UserID
	Name   string
}

type createTaskTagRepository interface {
	Create(ctx context.Context, tag domain.TaskTag) (dao.TaskTag, error)
}

type CreateTaskTagUseCase struct {
	repo   createTaskTagRepository
	logger logging.Logger
}

func NewCreateTaskTagUseCase(repo createTaskTagRepository, logger logging.Logger) *CreateTaskTagUseCase {
	return &CreateTaskTagUseCase{logger: logging.OrNop(logger), repo: repo}
}

func (uc *CreateTaskTagUseCase) Execute(ctx context.Context, input CreateTaskTagInput) (output dao.TaskTag, err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "CreateTaskTagUseCase.Execute", err) }()

	tag, err := domain.NewTaskTag(input.ID, input.UserID, input.Name)
	if err != nil {
		return dao.TaskTag{}, err
	}

	return uc.repo.Create(ctx, tag)

}
