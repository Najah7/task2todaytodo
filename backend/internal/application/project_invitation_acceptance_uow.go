package application

import (
	"context"

	authusecase "github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
	projectusecase "github.com/Najah7/task2todaytodo/internal/application/project/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProjectInvitationAcceptanceUOW struct {
	pool     *pgxpool.Pool
	projects ProjectStore
	auth     AuthStore
}

func NewProjectInvitationAcceptanceUOW(pool *pgxpool.Pool, projects ProjectStore, auth AuthStore) projectusecase.InvitationAcceptanceUnitOfWork {
	return &ProjectInvitationAcceptanceUOW{pool: pool, projects: projects, auth: auth}
}

func (uow *ProjectInvitationAcceptanceUOW) Do(ctx context.Context, fn func(context.Context, projectusecase.InvitationAcceptanceRepositories) error) error {
	return RunInTx(ctx, uow.pool, func(tx pgx.Tx) error {
		return fn(ctx, projectInvitationAcceptanceRepositories{
			invitations: uow.projects.WithTx(tx).Projects,
			recipient: projectInvitationRecipientAdapter{
				repositories: authRepositories{store: uow.auth.WithTx(tx)},
			},
		})
	})
}

type projectInvitationAcceptanceRepositories struct {
	invitations projectusecase.InvitationAcceptanceRepository
	recipient   projectusecase.InvitationRecipientAcceptor
}

func (repositories projectInvitationAcceptanceRepositories) Invitations() projectusecase.InvitationAcceptanceRepository {
	return repositories.invitations
}

func (repositories projectInvitationAcceptanceRepositories) Recipient() projectusecase.InvitationRecipientAcceptor {
	return repositories.recipient
}

type projectInvitationRecipientAdapter struct {
	repositories authusecase.Repositories
}

func (adapter projectInvitationRecipientAdapter) AcceptInvitationRecipient(ctx context.Context, email, password, authenticatedUserID, newUserID string) (projectusecase.InvitationRecipientResult, error) {
	accepted, err := authusecase.AcceptInvitationRecipient(ctx, adapter.repositories, newUserID, email, password, authenticatedUserID)
	if err != nil {
		if err == authusecase.ErrInvitationRecipientAuthenticationRequired {
			return projectusecase.InvitationRecipientResult{}, projectusecase.ErrInvitationRecipientAuthenticationRequired
		}
		return projectusecase.InvitationRecipientResult{}, err
	}
	return projectusecase.InvitationRecipientResult{
		UserID:              accepted.User.ID,
		PersonalAccessToken: accepted.PersonalAccessToken.Token,
		ExpiresAt:           accepted.PersonalAccessToken.ExpiresAt,
		NeedsAutoLogin:      accepted.NeedsAutoLogin,
	}, nil
}

var _ projectusecase.InvitationAcceptanceUnitOfWork = (*ProjectInvitationAcceptanceUOW)(nil)
