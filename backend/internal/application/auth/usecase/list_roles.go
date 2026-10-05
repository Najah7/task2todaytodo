package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/auth/dao"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type listRolesRepository interface {
	ListRoles(context.Context) ([]dao.Role, error)
}

type ListRolesUseCase struct {
	repo   listRolesRepository
	logger logging.Logger
}

func NewListRolesUseCase(repo listRolesRepository, logger logging.Logger) *ListRolesUseCase {
	return &ListRolesUseCase{repo: repo, logger: logging.OrNop(logger)}
}

func (uc *ListRolesUseCase) Execute(ctx context.Context) (roles []dao.Role, err error) {
	defer func() { logUnexpectedFailure(uc.logger, ctx, "ListRolesUseCase.Execute", err) }()
	return uc.repo.ListRoles(ctx)
}
