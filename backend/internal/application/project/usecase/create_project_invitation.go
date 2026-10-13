package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

type CreateProjectInvitationUseCase struct {
	uow      UnitOfWork
	sender   ProjectInvitationSender
	frontend string
	logger   telemetry.Logger
	now      func() time.Time
}

func NewCreateProjectInvitationUseCase(uow UnitOfWork, sender ProjectInvitationSender, frontend string, logger telemetry.Logger) *CreateProjectInvitationUseCase {
	return &CreateProjectInvitationUseCase{uow: uow, sender: sender, frontend: frontend, logger: telemetry.OrNop(logger), now: time.Now}
}

func (uc *CreateProjectInvitationUseCase) Execute(ctx context.Context, actor domain.UserID, project domain.ProjectID, id, emailValue, roleID string) (dao.ProjectInvitation, error) {
	email, err := normalizeInvitationEmail(emailValue)
	if err != nil {
		return dao.ProjectInvitation{}, err
	}
	if !validProjectMemberInput(string(actor), string(project), id, roleID) || (roleID != "admin" && roleID != "editor" && roleID != "viewer") {
		return dao.ProjectInvitation{}, ErrProjectInvitationInvalidInput
	}
	rawToken, tokenHash, err := newProjectInvitationToken()
	if err != nil {
		return dao.ProjectInvitation{}, err
	}
	now := uc.now().UTC()
	var invitation dao.ProjectInvitation
	var projectTitle string
	err = uc.uow.Do(ctx, func(ctx context.Context, repo Repository, _ ProjectChildrenDeleter) error {
		if err := repo.LockProjectForMemberChange(ctx, project); err != nil {
			return err
		}
		if _, err := repo.LockByUserIDWithPermission(ctx, actor, project, shared.ProjectMemberCreate()); err != nil {
			return err
		}
		actorRole, err := repo.GetProjectMemberActorRole(ctx, actor, project)
		if err != nil {
			return err
		}
		if !CanManageProjectMemberRole(actorRole, roleID) {
			return ErrPermissionDenied
		}
		if err := requireTeamInvitation(ctx, repo, actor, project); err != nil {
			return err
		}
		if err := reserveProjectInvitationEmail(ctx, repo, email, now); err != nil {
			return err
		}
		invitation, err = repo.CreateProjectInvitation(ctx, actor, project, id, email, roleID, tokenHash, now.Add(ProjectInvitationLifetime))
		if err != nil {
			return err
		}
		projectRecord, err := repo.GetByUserIDWithPermission(ctx, actor, project, shared.ProjectMemberRead())
		if err != nil {
			return err
		}
		projectTitle = projectRecord.Title
		return nil
	})
	if err != nil {
		return dao.ProjectInvitation{}, err
	}
	uc.send(ctx, invitation.Email, rawToken, projectTitle, invitation.RoleID)
	return invitation, nil
}

func (uc *CreateProjectInvitationUseCase) send(ctx context.Context, to, rawToken, title, role string) {
	if uc.sender == nil {
		return
	}
	sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := uc.sender.SendProjectInvitation(sendCtx, to, uc.frontend, rawToken, title, invitationRoleName(role), ProjectInvitationLifetime); err != nil {
		uc.logger.Warn(ctx, "project invitation email delivery failed", "operation", "project_invitation.send", "reason", "provider_error")
	}
	rawToken = ""
}

func reserveProjectInvitationEmail(ctx context.Context, repo Repository, email string, now time.Time) error {
	hash := invitationEmailHash(email)
	if err := repo.LockInvitationEmailDelivery(ctx, hash); err != nil {
		return err
	}
	if err := repo.DeleteExpiredInvitationEmailDeliveryAttempts(ctx, hash, now.Add(-projectInvitationSendWindow)); err != nil {
		return err
	}
	attempts, err := repo.CountInvitationEmailDeliveryAttemptsSince(ctx, hash, now.Add(-projectInvitationSendWindow))
	if err != nil {
		return err
	}
	recent, err := repo.CountInvitationEmailDeliveryAttemptsSince(ctx, hash, now.Add(-projectInvitationSendCooldown))
	if err != nil {
		return err
	}
	if attempts >= projectInvitationSendLimit || recent > 0 {
		return ErrProjectInvitationRateLimited
	}
	return repo.CreateInvitationEmailDeliveryAttempt(ctx, hash, now)
}
