package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type ListProjectTypesUseCase struct {
	repo interface {
		ListProjectTypes(context.Context) ([]dao.ProjectType, error)
	}
	logger telemetry.Logger
}

func NewListProjectTypesUseCase(repo interface {
	ListProjectTypes(context.Context) ([]dao.ProjectType, error)
}, logger telemetry.Logger) *ListProjectTypesUseCase {
	return &ListProjectTypesUseCase{repo: repo, logger: telemetry.OrNop(logger)}
}
func (uc *ListProjectTypesUseCase) Execute(ctx context.Context) (types []dao.ProjectType, err error) {
	defer func() { logUnexpectedProjectFailure(uc.logger, ctx, "ListProjectTypesUseCase.Execute", err) }()
	return uc.repo.ListProjectTypes(ctx)
}
