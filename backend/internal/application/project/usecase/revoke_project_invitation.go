package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type RevokeProjectInvitationUseCase struct {
	uow    UnitOfWork
	logger telemetry.Logger
}

func NewRevokeProjectInvitationUseCase(uow UnitOfWork, logger telemetry.Logger) *RevokeProjectInvitationUseCase {
	return &RevokeProjectInvitationUseCase{uow: uow, logger: telemetry.OrNop(logger)}
}

func (uc *RevokeProjectInvitationUseCase) Execute(ctx context.Context, actor domain.UserID, project domain.ProjectID, invitationID string) error {
	if !validProjectMemberInput(string(actor), string(project), invitationID) {
		return ErrProjectInvitationInvalidInput
	}
	return uc.uow.Do(ctx, func(ctx context.Context, repo Repository, _ ProjectChildrenDeleter) error {
		if err := repo.LockProjectForMemberChange(ctx, project); err != nil {
			return err
		}
		return repo.RevokeProjectInvitation(ctx, actor, project, invitationID)
	})
}
