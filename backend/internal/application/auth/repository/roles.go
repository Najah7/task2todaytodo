package repository

import (
	"context"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/auth/dao"
	"github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
	"github.com/jackc/pgx/v5"
)

var _ usecase.RoleCatalogRepository = RoleRepository{}

type RoleRepository struct {
	queries *sqlc.Queries
}

func NewRoleRepository(db sqlc.DBTX) *RoleRepository {
	return &RoleRepository{queries: sqlc.New(db)}
}

func (r *RoleRepository) WithTx(tx pgx.Tx) *RoleRepository {
	return &RoleRepository{queries: r.queries.WithTx(tx)}
}

func (r RoleRepository) ListRoles(ctx context.Context) ([]dao.Role, error) {
	rows, err := r.queries.ListRolesWithPermissions(ctx)
	if err != nil {
		return nil, err
	}
	roles := make([]dao.Role, 0)
	roleIndex := make(map[string]int)
	for _, row := range rows {
		index, exists := roleIndex[row.RoleID]
		if !exists {
			index = len(roles)
			roleIndex[row.RoleID] = index
			roles = append(roles, dao.Role{ID: row.RoleID, Name: row.Name, Permissions: make([]dao.Permission, 0)})
		}
		if !row.PermissionID.Valid {
			continue
		}
		roles[index].Permissions = append(roles[index].Permissions, dao.Permission{
			ID: row.PermissionID.Int64, ResourceID: row.ResourceID.String, ResourceName: row.ResourceName.String,
			Action: row.Action, Effect: row.Effect, Description: row.Description.String,
		})
	}
	return roles, nil
}

func (r RoleRepository) ListPermissions(ctx context.Context) ([]dao.Permission, error) {
	rows, err := r.queries.ListPermissions(ctx)
	if err != nil {
		return nil, err
	}
	permissions := make([]dao.Permission, 0, len(rows))
	for _, row := range rows {
		permissions = append(permissions, dao.Permission{
			ID: row.PermissionID, ResourceID: row.ResourceID, ResourceName: row.ResourceName,
			Action: row.Action, Effect: row.Effect, Description: row.Description,
		})
	}
	return permissions, nil
}
