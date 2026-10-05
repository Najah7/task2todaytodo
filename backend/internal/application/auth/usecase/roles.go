package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/auth/dao"
)

type RoleCatalogRepository interface {
	ListRoles(ctx context.Context) ([]dao.Role, error)
	ListPermissions(ctx context.Context) ([]dao.Permission, error)
}

type RoleUseCases struct {
	List        *ListRolesUseCase
	Permissions *ListPermissionsUseCase
}
