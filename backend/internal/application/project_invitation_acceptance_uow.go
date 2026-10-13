package application

import (
	"context"

	authusecase "github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
	billingrepository "github.com/Najah7/task2todaytodo/internal/application/billing/repository"
	projectusecase "github.com/Najah7/task2todaytodo/internal/application/project/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProjectInvitationAcceptanceUOW struct {
	pool           *pgxpool.Pool
	projects       ProjectStore
	auth           AuthStore
	billing        *billingrepository.Repository
	billingEnabled bool
}

func NewProjectInvitationAcceptanceUOW(pool *pgxpool.Pool, projects ProjectStore, auth AuthStore) projectusecase.InvitationAcceptanceUnitOfWork {
	return &ProjectInvitationAcceptanceUOW{pool: pool, projects: projects, auth: auth}
}

func (uow *ProjectInvitationAcceptanceUOW) Do(ctx context.Context, fn func(context.Context, projectusecase.InvitationAcceptanceRepositories) error) error {
	return RunInTx(ctx, uow.pool, func(tx pgx.Tx) error {
		projectRows := uow.projects.WithTx(tx).Projects
		access := newProjectWriteAccessReader(projectRows, billingWithTx(uow.billing, tx), uow.billingEnabled)
		return fn(ctx, projectInvitationAcceptanceRepositories{
			invitations: projectInvitationRepositoryWithWriteAccess{ProjectRepository: projectRows, ProjectWriteAccessReader: access},
			recipient: projectInvitationRecipientAdapter{
				repositories: authRepositories{store: uow.auth.WithTx(tx)},
			},
		})
	})
}

func (uow *ProjectInvitationAcceptanceUOW) ConfigureProjectWriteAccess(billing *billingrepository.Repository, enabled bool) {
	uow.billing = billing
	uow.billingEnabled = enabled
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
