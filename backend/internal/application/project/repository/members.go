package repository

import (
	"context"
	"errors"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	projectusecase "github.com/Najah7/task2todaytodo/internal/application/project/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (r *ProjectRepository) LockProjectForMemberChange(ctx context.Context, id domain.ProjectID) error {
	_, err := r.queries.LockProjectForMemberChange(ctx, string(id))
	if err != nil {
		return repoError(err)
	}
	return nil
}
func (r *ProjectRepository) CheckProjectMemberUpsertPermission(ctx context.Context, actor domain.UserID, project domain.ProjectID, member domain.UserID) (bool, error) {
	ok, err := r.queries.HasProjectMemberUpsertPermission(ctx, sqlc.HasProjectMemberUpsertPermissionParams{ActorID: string(actor), ProjectID: string(project), MemberID: string(member)})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, domain.ErrProjectNotFound
	}
	return ok, err
}
func (r *ProjectRepository) UpsertProjectMember(ctx context.Context, m domain.ProjectMember) error {
	n, err := r.queries.UpsertProjectMember(ctx, sqlc.UpsertProjectMemberParams{MemberID: string(m.UserID), RoleID: m.RoleID, ActorID: string(m.AddedBy), ProjectID: string(m.ProjectID)})
	if err != nil {
		return memberError(err)
	}
	if n == 0 {
		ok, e := r.CheckProjectMemberUpsertPermission(ctx, m.AddedBy, m.ProjectID, m.UserID)
		if e != nil {
			return e
		}
		if !ok {
			return projectusecase.ErrPermissionDenied
		}
		return projectusecase.ErrProjectMemberUpsertRejected
	}
	return nil
}
func (r *ProjectRepository) DeleteProjectMember(ctx context.Context, actor domain.UserID, project domain.ProjectID, member domain.UserID) error {
	n, err := r.queries.DeleteProjectMember(ctx, sqlc.DeleteProjectMemberParams{ProjectID: string(project), MemberID: string(member), ActorID: string(actor)})
	if err != nil {
		return err
	}
	if n == 0 {
		return projectusecase.ErrProjectMemberNotFound
	}
	return nil
}
func (r *ProjectRepository) ListProjectMembers(ctx context.Context, actor domain.UserID, project domain.ProjectID) ([]dao.ProjectMember, error) {
	rows, err := r.queries.ListProjectMembers(ctx, sqlc.ListProjectMembersParams{ProjectID: string(project), ActorID: string(actor)})
	if err != nil {
		return nil, err
	}
	out := make([]dao.ProjectMember, 0, len(rows))
	for _, x := range rows {
		out = append(out, dao.ProjectMember{ProjectID: x.ProjectID, UserID: x.UserID, RoleID: x.RoleID, RoleName: x.RoleName, FirstName: x.FirstName, LastName: x.LastName, Email: x.Email, AddedBy: x.AddedBy, CreatedAt: x.CreatedAt.Time.Unix(), UpdatedAt: x.UpdatedAt.Time.Unix()})
	}
	return out, nil
}
func memberError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
		return err
	}
	switch pgErr.ConstraintName {
	case "project_members_role_id_fkey":
		return projectusecase.ErrProjectMemberRoleNotFound
	case "project_members_user_id_fkey":
		return projectusecase.ErrProjectMemberUserNotFound
	}
	return err
}
