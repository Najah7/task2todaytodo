package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

type projectPermissionReader interface {
	HasPermission(context.Context, domain.UserID, domain.ProjectID, shared.Capability) (bool, error)
}

func readProjectCapabilities(ctx context.Context, reader projectPermissionReader, actor domain.UserID, project *dao.Project) error {
	var err error
	project.CanUpdate, err = reader.HasPermission(ctx, actor, domain.ProjectID(project.ID), shared.ProjectUpdate())
	if err != nil {
		return err
	}
	project.CanDelete, err = reader.HasPermission(ctx, actor, domain.ProjectID(project.ID), shared.ProjectDelete())
	return err
}
