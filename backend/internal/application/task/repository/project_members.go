package repository

import (
	"context"
	"errors"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/jackc/pgx/v5/pgconn"
)

func (r ProjectRepository) LockProjectForMemberChange(ctx context.Context, project domain.ProjectID) error {
	_, err := r.queries.LockProjectForMemberChange(ctx, string(project))
	if err != nil {
		return projectRepositoryError(err)
	}
	return nil
}

func (r ProjectRepository) CheckProjectMemberUpsertPermission(ctx context.Context, actor domain.UserID, project domain.ProjectID, member domain.UserID) (bool, error) {
	allowed, err := r.queries.HasProjectMemberUpsertPermission(ctx, sqlc.HasProjectMemberUpsertPermissionParams{
		ActorID: string(actor), ProjectID: string(project), MemberID: string(member),
	})
	if err != nil {
		return false, projectRepositoryError(err)
	}
	return allowed, nil
}

func (r ProjectRepository) UpsertProjectMember(ctx context.Context, member domain.ProjectMember) error {
	rows, err := r.queries.UpsertProjectMember(ctx, sqlc.UpsertProjectMemberParams{
		MemberID:  string(member.UserID),
		RoleID:    member.RoleID,
		ActorID:   string(member.AddedBy),
		ProjectID: string(member.ProjectID),
	})
	if err != nil {
		return projectMemberRepositoryError(err)
	}
	if rows == 0 {
		allowed, err := r.CheckProjectMemberUpsertPermission(ctx, member.AddedBy, member.ProjectID, member.UserID)
		if err != nil {
			return err
		}
		if !allowed {
			return usecase.ErrPermissionDenied
		}
		return usecase.ErrProjectMemberUpsertRejected
	}
	return nil
}

func projectMemberRepositoryError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
		return err
	}
	switch pgErr.ConstraintName {
	case "project_members_role_id_fkey":
		return usecase.ErrProjectMemberRoleNotFound
	case "project_members_user_id_fkey":
		return usecase.ErrProjectMemberUserNotFound
	default:
		return err
	}
}

func (r TaskRepository) ReassignProjectMemberTasks(ctx context.Context, actor domain.UserID, project domain.ProjectID, member domain.UserID) error {
	_, err := r.queries.ReassignProjectMemberTasks(ctx, sqlc.ReassignProjectMemberTasksParams{
		ActorID:   string(actor),
		ProjectID: string(project),
		MemberID:  string(member),
	})
	if err != nil {
		return err
	}
	return nil
}

func (r ProjectRepository) DeleteProjectMember(ctx context.Context, actor domain.UserID, project domain.ProjectID, member domain.UserID) error {
	rows, err := r.queries.DeleteProjectMember(ctx, sqlc.DeleteProjectMemberParams{
		ProjectID: string(project),
		MemberID:  string(member),
		ActorID:   string(actor),
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		allowed, checkErr := r.HasPermission(ctx, actor, project, shared.ProjectMemberDelete())
		if checkErr != nil {
			return checkErr
		}
		if !allowed {
			return usecase.ErrPermissionDenied
		}
		return usecase.ErrProjectMemberNotFound
	}
	return nil
}

func (r ProjectRepository) ListProjectMembers(ctx context.Context, actor domain.UserID, project domain.ProjectID) ([]dao.ProjectMember, error) {
	rows, err := r.queries.ListProjectMembers(ctx, sqlc.ListProjectMembersParams{
		ProjectID: string(project),
		ActorID:   string(actor),
	})
	if err != nil {
		return nil, err
	}
	members := make([]dao.ProjectMember, 0, len(rows))
	for _, row := range rows {
		members = append(members, dao.ProjectMember{
			ProjectID: row.ProjectID,
			UserID:    row.UserID,
			RoleID:    row.RoleID,
			RoleName:  row.RoleName,
			FirstName: row.FirstName,
			LastName:  row.LastName,
			Email:     row.Email,
			AddedBy:   row.AddedBy,
			CreatedAt: row.CreatedAt.Time.Unix(),
			UpdatedAt: row.UpdatedAt.Time.Unix(),
		})
	}
	return members, nil
}
