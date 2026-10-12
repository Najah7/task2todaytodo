package repository

import (
	"context"
	"errors"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	projectusecase "github.com/Najah7/task2todaytodo/internal/application/project/usecase"
	"github.com/jackc/pgx/v5"
)

func (r *ProjectRepository) GetProjectInvitationByTokenHash(ctx context.Context, hash []byte) (dao.ProjectInvitation, error) {
	row, err := r.queries.GetProjectInvitationByTokenHash(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return dao.ProjectInvitation{}, projectusecase.ErrProjectInvitationTokenInvalid
	}
	if err != nil {
		return dao.ProjectInvitation{}, repoError(err)
	}
	return invitationRecordToDAO(row), nil
}

func (r *ProjectRepository) GetProjectInvitationByTokenHashForUpdate(ctx context.Context, hash []byte) (dao.ProjectInvitation, error) {
	row, err := r.queries.GetProjectInvitationByTokenHashForUpdate(ctx, hash)
	if err != nil {
		return dao.ProjectInvitation{}, repoError(err)
	}
	return invitationRecordForUpdateToDAO(row), nil
}

func (r *ProjectRepository) LockProjectForInvitationAcceptance(ctx context.Context, project domain.ProjectID) error {
	_, err := r.queries.LockProjectForInvitationAcceptance(ctx, string(project))
	return repoError(err)
}

func (r *ProjectRepository) ConsumeProjectInvitation(ctx context.Context, hash []byte) (bool, error) {
	count, err := r.queries.ConsumeProjectInvitation(ctx, hash)
	return count > 0, err
}

func (r *ProjectRepository) AcceptProjectInvitationMember(ctx context.Context, project domain.ProjectID, user domain.UserID, roleID, invitedBy string) (bool, error) {
	count, err := r.queries.AcceptProjectInvitationMember(ctx, sqlc.AcceptProjectInvitationMemberParams{
		ProjectID: string(project), UserID: string(user), RoleID: roleID, InvitedBy: invitedBy,
	})
	return count > 0, err
}
