package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	"github.com/Najah7/task2todaytodo/internal/application/project/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (r *ProjectRepository) ListProjectRosterPage(ctx context.Context, actor domain.UserID, project domain.ProjectID, limit int, anchor *usecase.CursorAnchor) (dao.ProjectRosterPage, error) {
	direction := "forward"
	if anchor != nil && anchor.Direction == "backward" {
		direction = "backward"
	}
	arg := sqlc.ListProjectRosterPageParams{ProjectID: string(project), ActorID: string(actor), PageLimit: int32(limit), CursorDirection: direction}
	if anchor != nil {
		at, err := time.Parse(time.RFC3339Nano, anchor.At)
		if err != nil || anchor.ID == "" {
			return dao.ProjectRosterPage{}, usecase.ErrInvalidProjectRosterPage
		}
		arg.CursorAt = pgtype.Timestamptz{Time: at, Valid: true}
		arg.CursorID = pgtype.Text{String: anchor.ID, Valid: true}
	}
	rows, err := r.queries.ListProjectRosterPage(ctx, arg)
	if err != nil {
		return dao.ProjectRosterPage{}, err
	}
	page := dao.ProjectRosterPage{Items: make([]dao.ProjectMember, 0, len(rows))}
	for i, row := range rows {
		if i == 0 {
			page.TotalCount = row.TotalCount
			page.MemberCount = row.MemberCount
			page.PendingInvitationCount = row.PendingInvitationCount
		}
		page.Items = append(page.Items, dao.ProjectMember{
			ProjectID: row.ProjectID, UserID: row.UserID, RoleID: row.RoleID, RoleName: row.RoleName,
			FirstName: row.FirstName, LastName: row.LastName, Email: row.Email, AddedBy: row.AddedBy,
			CreatedAt: row.CreatedAt.Time.Unix(), UpdatedAt: row.UpdatedAt.Time.Unix(), Status: row.Status,
			InvitationID: row.InvitationID, ExpiresAt: nullableUnix(row.ExpiresAt), CanEditRole: row.CanEditRole,
			CanRemove: row.CanRemove, CursorCreatedAt: row.CreatedAt.Time.UTC().Format(time.RFC3339Nano), CursorID: row.SortID,
		})
	}
	return page, nil
}

func (r *ProjectRepository) GetProjectMemberActorRole(ctx context.Context, actor domain.UserID, project domain.ProjectID) (string, error) {
	value, err := r.queries.GetProjectMemberActorRole(ctx, sqlc.GetProjectMemberActorRoleParams{ProjectID: string(project), ActorID: string(actor)})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrProjectNotFound
	}
	if err != nil {
		return "", err
	}
	role, ok := value.(string)
	if !ok {
		return "", errors.New("project member actor role has unexpected database type")
	}
	return role, nil
}

func nullableUnix(value pgtype.Timestamptz) int64 {
	if !value.Valid {
		return 0
	}
	return value.Time.Unix()
}

func invitationRecordToDAO(row sqlc.GetProjectInvitationByTokenHashRow) dao.ProjectInvitation {
	return dao.ProjectInvitation{
		ID: row.ID, ProjectID: row.ProjectID, Email: row.Email, RoleID: row.RoleID, InvitedBy: row.InvitedBy,
		ExpiresAt: row.ExpiresAt.Time, UsedAt: timestampOrZero(row.UsedAt), RevokedAt: timestampOrZero(row.RevokedAt),
		ProjectTitle: row.ProjectTitle, ProjectOwnerID: row.ProjectOwnerID, ProjectDeletedAt: timestampOrZero(row.DeletedAt),
		InviterRoleID: row.InviterRoleID, RecipientStatus: row.RecipientStatus,
	}
}

func invitationRecordForUpdateToDAO(row sqlc.GetProjectInvitationByTokenHashForUpdateRow) dao.ProjectInvitation {
	return dao.ProjectInvitation{
		ID: row.ID, ProjectID: row.ProjectID, Email: row.Email, RoleID: row.RoleID, InvitedBy: row.InvitedBy,
		ExpiresAt: row.ExpiresAt.Time, UsedAt: timestampOrZero(row.UsedAt), RevokedAt: timestampOrZero(row.RevokedAt),
		ProjectTitle: row.ProjectTitle, ProjectOwnerID: row.ProjectOwnerID, ProjectDeletedAt: timestampOrZero(row.DeletedAt),
		InviterRoleID: row.InviterRoleID, RecipientStatus: row.RecipientStatus,
	}
}

func timestampOrZero(value pgtype.Timestamptz) time.Time {
	if !value.Valid {
		return time.Time{}
	}
	return value.Time
}

func (r *ProjectRepository) GetProjectInvitationForResend(ctx context.Context, project domain.ProjectID, invitationID string) (dao.ProjectInvitation, error) {
	row, err := r.queries.GetProjectInvitationForResend(ctx, sqlc.GetProjectInvitationForResendParams{ProjectID: string(project), InvitationID: invitationID})
	if errors.Is(err, pgx.ErrNoRows) {
		return dao.ProjectInvitation{}, usecase.ErrProjectInvitationNotFound
	}
	if err != nil {
		return dao.ProjectInvitation{}, err
	}
	return dao.ProjectInvitation{
		ID: row.ID, ProjectID: row.ProjectID, Email: row.Email, RoleID: row.RoleID, InvitedBy: row.InvitedBy,
		ExpiresAt: row.ExpiresAt.Time, UsedAt: timestampOrZero(row.UsedAt), RevokedAt: timestampOrZero(row.RevokedAt), ProjectTitle: row.ProjectTitle,
	}, nil
}

func (r *ProjectRepository) CreateProjectInvitation(ctx context.Context, actor domain.UserID, project domain.ProjectID, id, email, roleID string, tokenHash []byte, expiresAt time.Time) (dao.ProjectInvitation, error) {
	row, err := r.queries.CreateProjectInvitation(ctx, sqlc.CreateProjectInvitationParams{
		ID: id, ProjectID: string(project), ActorID: string(actor), Email: email, RoleID: roleID,
		TokenHash: tokenHash, ExpiresAt: pgTimestamp(expiresAt),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return dao.ProjectInvitation{}, usecase.ErrProjectInvitationRejected
	}
	if err != nil {
		return dao.ProjectInvitation{}, err
	}
	return dao.ProjectInvitation{ID: row.ID, ProjectID: row.ProjectID, Email: row.Email, RoleID: row.RoleID, InvitedBy: row.InvitedBy, ExpiresAt: row.ExpiresAt.Time}, nil
}

func (r *ProjectRepository) ResendProjectInvitation(ctx context.Context, actor domain.UserID, project domain.ProjectID, id string, tokenHash []byte, expiresAt time.Time) (dao.ProjectInvitation, error) {
	row, err := r.queries.ResendProjectInvitation(ctx, sqlc.ResendProjectInvitationParams{
		ProjectID: string(project), ActorID: string(actor), InvitationID: id, TokenHash: tokenHash, ExpiresAt: pgTimestamp(expiresAt),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return dao.ProjectInvitation{}, usecase.ErrProjectInvitationNotFound
	}
	if err != nil {
		return dao.ProjectInvitation{}, err
	}
	return dao.ProjectInvitation{ID: row.ID, ProjectID: row.ProjectID, Email: row.Email, RoleID: row.RoleID, InvitedBy: row.InvitedBy, ExpiresAt: row.ExpiresAt.Time}, nil
}

func (r *ProjectRepository) RevokeProjectInvitation(ctx context.Context, actor domain.UserID, project domain.ProjectID, id string) error {
	count, err := r.queries.RevokeProjectInvitation(ctx, sqlc.RevokeProjectInvitationParams{ProjectID: string(project), ActorID: string(actor), InvitationID: id})
	if err != nil {
		return err
	}
	if count == 0 {
		return usecase.ErrProjectInvitationNotFound
	}
	return nil
}

func (r *ProjectRepository) RevokeInvitationsForInviterWithoutPermission(ctx context.Context, project domain.ProjectID, inviter domain.UserID) error {
	_, err := r.queries.RevokeInvitationsForInviterWithoutPermission(ctx, sqlc.RevokeInvitationsForInviterWithoutPermissionParams{ProjectID: string(project), InviterID: string(inviter)})
	return err
}

func (r *ProjectRepository) LockInvitationEmailDelivery(ctx context.Context, hash []byte) error {
	return r.queries.LockEmailDeliveryAttempts(ctx, hash)
}

func (r *ProjectRepository) DeleteExpiredInvitationEmailDeliveryAttempts(ctx context.Context, hash []byte, cutoff time.Time) error {
	return r.queries.DeleteExpiredEmailDeliveryAttempts(ctx, sqlc.DeleteExpiredEmailDeliveryAttemptsParams{EmailHash: hash, AttemptedAt: pgTimestamp(cutoff)})
}

func (r *ProjectRepository) CountInvitationEmailDeliveryAttemptsSince(ctx context.Context, hash []byte, since time.Time) (int64, error) {
	return r.queries.CountRecentEmailDeliveryAttempts(ctx, sqlc.CountRecentEmailDeliveryAttemptsParams{EmailHash: hash, AttemptedAt: pgTimestamp(since)})
}

func (r *ProjectRepository) CreateInvitationEmailDeliveryAttempt(ctx context.Context, hash []byte, at time.Time) error {
	return r.queries.CreateEmailDeliveryAttempt(ctx, sqlc.CreateEmailDeliveryAttemptParams{EmailHash: hash, AttemptedAt: pgTimestamp(at)})
}

func pgTimestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}
