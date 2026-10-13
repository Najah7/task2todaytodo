package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type ResendProjectInvitationUseCase struct {
	uow      UnitOfWork
	sender   ProjectInvitationSender
	frontend string
	logger   telemetry.Logger
	now      func() time.Time
}

func NewResendProjectInvitationUseCase(uow UnitOfWork, sender ProjectInvitationSender, frontend string, logger telemetry.Logger) *ResendProjectInvitationUseCase {
	return &ResendProjectInvitationUseCase{uow: uow, sender: sender, frontend: frontend, logger: telemetry.OrNop(logger), now: time.Now}
}

func (uc *ResendProjectInvitationUseCase) Execute(ctx context.Context, actor domain.UserID, project domain.ProjectID, invitationID string) (dao.ProjectInvitation, error) {
	if !validProjectMemberInput(string(actor), string(project), invitationID) {
		return dao.ProjectInvitation{}, ErrProjectInvitationInvalidInput
	}
	rawToken, tokenHash, err := newProjectInvitationToken()
	if err != nil {
		return dao.ProjectInvitation{}, err
	}
	now := uc.now().UTC()
	var invitation dao.ProjectInvitation
	err = uc.uow.Do(ctx, func(ctx context.Context, repo Repository, _ ProjectChildrenDeleter) error {
		if err := repo.LockProjectForMemberChange(ctx, project); err != nil {
			return err
		}
		current, err := repo.GetProjectInvitationForResend(ctx, project, invitationID)
		if err != nil {
			return err
		}
		actorRole, err := repo.GetProjectMemberActorRole(ctx, actor, project)
		if err != nil {
			return err
		}
		if !CanManageProjectMemberRole(actorRole, current.RoleID) {
			return ErrPermissionDenied
		}
		if err := requireTeamInvitation(ctx, repo, actor, project); err != nil {
			return err
		}
		if err := reserveProjectInvitationEmail(ctx, repo, current.Email, now); err != nil {
			return err
		}
		invitation, err = repo.ResendProjectInvitation(ctx, actor, project, invitationID, tokenHash, now.Add(ProjectInvitationLifetime))
		if err != nil {
			return err
		}
		invitation.ProjectTitle = current.ProjectTitle
		return nil
	})
	if err != nil {
		return dao.ProjectInvitation{}, err
	}
	if uc.sender != nil {
		sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := uc.sender.SendProjectInvitation(sendCtx, invitation.Email, uc.frontend, rawToken, invitation.ProjectTitle, invitationRoleName(invitation.RoleID), ProjectInvitationLifetime); err != nil {
			uc.logger.Warn(ctx, "project invitation email delivery failed", "operation", "project_invitation.resend", "reason", "provider_error")
		}
	}
	rawToken = ""
	return invitation, nil
}
