package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type ListProjectTypesUseCase struct {
	repo interface {
		ListProjectTypes(context.Context) ([]dao.ProjectType, error)
	}
	logger logging.Logger
}

func NewListProjectTypesUseCase(repo interface {
	ListProjectTypes(context.Context) ([]dao.ProjectType, error)
}, logger logging.Logger) *ListProjectTypesUseCase {
	return &ListProjectTypesUseCase{repo: repo, logger: logging.OrNop(logger)}
}
func (uc *ListProjectTypesUseCase) Execute(ctx context.Context) (types []dao.ProjectType, err error) {
	defer func() { logUnexpectedProjectFailure(uc.logger, ctx, "ListProjectTypesUseCase.Execute", err) }()
	return uc.repo.ListProjectTypes(ctx)
}
