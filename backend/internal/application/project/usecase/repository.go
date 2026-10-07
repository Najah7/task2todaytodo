package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

type CursorAnchor struct {
	At, ID   string
	Revision int32
}
type CursorPageRequest struct {
	Size   int
	Anchor *CursorAnchor
}
type CursorPage[T any] struct {
	Items []T
	Next  *CursorAnchor
}

type Repository interface {
	ProjectProgressReader
	GetByUserID(context.Context, domain.UserID, domain.ProjectID) (dao.Project, error)
	GetByUserIDWithPermission(context.Context, domain.UserID, domain.ProjectID, shared.Capability) (dao.Project, error)
	LockByUserIDWithPermission(context.Context, domain.UserID, domain.ProjectID, shared.Capability) (dao.Project, error)
	HasPermission(context.Context, domain.UserID, domain.ProjectID, shared.Capability) (bool, error)
	ListByUserID(context.Context, domain.UserID) ([]dao.Project, error)
	ListByUserIDCursor(context.Context, domain.UserID, int, *CursorAnchor) ([]dao.Project, error)
	Create(context.Context, domain.Project) (dao.Project, error)
	UpdateByUserID(context.Context, domain.UserID, domain.Project, int32) (dao.Project, error)
	DeleteByUserID(context.Context, domain.UserID, domain.ProjectID, int32) error
	LockProjectForMemberChange(context.Context, domain.ProjectID) error
	CheckProjectMemberUpsertPermission(context.Context, domain.UserID, domain.ProjectID, domain.UserID) (bool, error)
	UpsertProjectMember(context.Context, domain.ProjectMember) error
	DeleteProjectMember(context.Context, domain.UserID, domain.ProjectID, domain.UserID) error
	ListProjectMembers(context.Context, domain.UserID, domain.ProjectID) ([]dao.ProjectMember, error)
	ListProjectRevisionsByActor(context.Context, domain.UserID, domain.ProjectID, int, *CursorAnchor) ([]dao.ProjectRevision, error)
	ListProjectTypes(context.Context) ([]dao.ProjectType, error)
}

type ProjectChildrenDeleter interface {
	DeleteProjectTasks(context.Context, string, string) error
	DeleteProjectSchedules(context.Context, string, string) error
	ReassignTasksAfterProjectMemberRemoval(context.Context, string, string, string) error
	ReassignSchedulesAfterProjectMemberRemoval(context.Context, string, string, string) error
}
type UnitOfWork interface {
	Do(context.Context, func(context.Context, Repository, ProjectChildrenDeleter) error) error
}
type UOW = UnitOfWork
type DeleteProjectUnitOfWork = UnitOfWork
