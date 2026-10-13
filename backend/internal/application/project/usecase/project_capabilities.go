package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

type projectReadUnitOfWork interface {
	Do(context.Context, func(context.Context, Repository, ProjectChildrenDeleter) error) error
}

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
	if err != nil {
		return err
	}
	if accessReader, ok := reader.(shared.ProjectWriteAccessReader); ok {
		access, err := accessReader.GetProjectWriteAccess(ctx, string(actor), project.ID)
		if err != nil {
			return err
		}
		if !access.AllowsWrite() {
			project.CanUpdate = false
			project.CanDelete = false
		}
	}
	return nil
}

func restrictProjectWriteCapabilities(ctx context.Context, uow projectReadUnitOfWork, actor domain.UserID, projects []dao.Project) error {
	if len(projects) == 0 {
		return nil
	}
	if uow == nil {
		return ErrProjectWriteAccessUnavailable
	}
	return uow.Do(ctx, func(ctx context.Context, repo Repository, _ ProjectChildrenDeleter) error {
		reader, ok := repo.(shared.ProjectWriteAccessReader)
		if !ok || reader == nil {
			return ErrProjectWriteAccessUnavailable
		}
		for i := range projects {
			access, err := reader.GetProjectWriteAccess(ctx, string(actor), projects[i].ID)
			if err != nil {
				return err
			}
			if !access.AllowsWrite() {
				projects[i].CanUpdate = false
				projects[i].CanDelete = false
			}
		}
		return nil
	})
}
