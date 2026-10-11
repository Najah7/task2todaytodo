package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/auth/dao"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type listPermissionsRepository interface {
	ListPermissions(context.Context) ([]dao.Permission, error)
}

type ListPermissionsUseCase struct {
	repo   listPermissionsRepository
	logger telemetry.Logger
}

func NewListPermissionsUseCase(repo listPermissionsRepository, logger telemetry.Logger) *ListPermissionsUseCase {
	return &ListPermissionsUseCase{repo: repo, logger: telemetry.OrNop(logger)}
}

func (uc *ListPermissionsUseCase) Execute(ctx context.Context) (permissions []dao.Permission, err error) {
	defer func() { logUnexpectedFailure(uc.logger, ctx, "ListPermissionsUseCase.Execute", err) }()
	return uc.repo.ListPermissions(ctx)
}
