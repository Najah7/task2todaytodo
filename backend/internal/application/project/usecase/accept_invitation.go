package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

var (
	ErrInvalidProjectInvitation                  = errors.New("project invitation is invalid or expired")
	ErrProjectInvitationMemberExists             = errors.New("invited user is already a project member")
	ErrInvitationRecipientAuthenticationRequired = errors.New("invitation recipient must authenticate with the invited account")
	ErrInvitationTeamSubscriptionRequired        = shared.ErrTeamSubscriptionRequired
)

type InvitationAcceptanceRepository interface {
	shared.ProjectWriteAccessReader
	GetProjectInvitationByTokenHash(context.Context, []byte) (dao.ProjectInvitation, error)
	GetProjectInvitationByTokenHashForUpdate(context.Context, []byte) (dao.ProjectInvitation, error)
	LockProjectForInvitationAcceptance(context.Context, domain.ProjectID) error
	ConsumeProjectInvitation(context.Context, []byte) (bool, error)
	AcceptProjectInvitationMember(context.Context, domain.ProjectID, domain.UserID, string, string) (bool, error)
}

type InvitationRecipientResult struct {
	UserID                   string
	PersonalAccessToken      string
	ExpiresAt                int64
	NeedsAutoLogin           bool
	RequiresTeamSubscription bool
}

type InvitationRecipientAcceptor interface {
	AcceptInvitationRecipient(context.Context, string, string, string, string) (InvitationRecipientResult, error)
}

type InvitationAcceptanceRepositories interface {
	Invitations() InvitationAcceptanceRepository
	Recipient() InvitationRecipientAcceptor
}

type InvitationAcceptanceUnitOfWork interface {
	Do(context.Context, func(context.Context, InvitationAcceptanceRepositories) error) error
}

type AcceptProjectInvitationUseCase struct {
	uow    InvitationAcceptanceUnitOfWork
	logger telemetry.Logger
	now    func() time.Time
}

func NewAcceptProjectInvitationUseCase(uow InvitationAcceptanceUnitOfWork, logger telemetry.Logger) *AcceptProjectInvitationUseCase {
	return &AcceptProjectInvitationUseCase{uow: uow, logger: telemetry.OrNop(logger), now: time.Now}
}

func (uc *AcceptProjectInvitationUseCase) Execute(ctx context.Context, rawToken, password, authenticatedUserID, newUserID string) (InvitationRecipientResult, error) {
	hash, ok := projectInvitationTokenHash(rawToken)
	if !ok {
		return InvitationRecipientResult{}, ErrInvalidProjectInvitation
	}
	var result InvitationRecipientResult
	err := uc.uow.Do(ctx, func(ctx context.Context, repositories InvitationAcceptanceRepositories) error {
		initial, err := repositories.Invitations().GetProjectInvitationByTokenHash(ctx, hash)
		if telemetry.IsNotFound(err) {
			return ErrInvalidProjectInvitation
		}
		if err != nil {
			return err
		}
		if err := repositories.Invitations().LockProjectForInvitationAcceptance(ctx, domain.ProjectID(initial.ProjectID)); err != nil {
			if telemetry.IsNotFound(err) {
				return ErrInvalidProjectInvitation
			}
			return err
		}
		invitation, err := repositories.Invitations().GetProjectInvitationByTokenHashForUpdate(ctx, hash)
		if telemetry.IsNotFound(err) {
			return ErrInvalidProjectInvitation
		}
		if err != nil {
			return err
		}
		if invitation.ProjectID != initial.ProjectID {
			return ErrInvalidProjectInvitation
		}
		now := uc.now()
		if invitation.UsedAt.IsZero() == false || !invitation.RevokedAt.IsZero() || !now.Before(invitation.ExpiresAt) {
			return ErrInvalidProjectInvitation
		}
		if !validInvitationMemberRole(invitation.RoleID) {
			return ErrInvalidProjectInvitation
		}
		if !invitation.ProjectDeletedAt.IsZero() {
			return ErrInvalidProjectInvitation
		}
		if !invitationIssuerAuthorized(invitation) {
			return ErrInvalidProjectInvitation
		}
		if invitation.ProjectOwnerID == authenticatedUserID && authenticatedUserID != "" {
			return ErrInvalidProjectInvitation
		}
		if err := requireTeamProjectOwner(ctx, repositories.Invitations(), domain.UserID(invitation.ProjectOwnerID), domain.ProjectID(invitation.ProjectID)); err != nil {
			return err
		}
		recipient, err := repositories.Recipient().AcceptInvitationRecipient(ctx, invitation.Email, password, authenticatedUserID, newUserID)
		if errors.Is(err, ErrInvitationRecipientAuthenticationRequired) {
			return ErrInvitationRecipientAuthenticationRequired
		}
		if err != nil {
			return err
		}
		if invitation.RoleID == "admin" || invitation.RoleID == "editor" {
			access, err := repositories.Invitations().GetProjectWriteAccess(ctx, recipient.UserID, invitation.ProjectID)
			if err != nil {
				return err
			}
			if !access.ActorPlanAccess.IsTeam() {
				if recipient.NeedsAutoLogin {
					result = recipient
					result.RequiresTeamSubscription = true
					return nil
				}
				return ErrInvitationTeamSubscriptionRequired
			}
		}
		added, err := repositories.Invitations().AcceptProjectInvitationMember(ctx, domain.ProjectID(invitation.ProjectID), domain.UserID(recipient.UserID), invitation.RoleID, invitation.InvitedBy)
		if err != nil {
			return err
		}
		if !added {
			return ErrProjectInvitationMemberExists
		}
		consumed, err := repositories.Invitations().ConsumeProjectInvitation(ctx, hash)
		if err != nil {
			return err
		}
		if !consumed {
			return ErrInvalidProjectInvitation
		}
		result = recipient
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrInvalidProjectInvitation) || errors.Is(err, ErrInvitationRecipientAuthenticationRequired) || errors.Is(err, ErrProjectInvitationMemberExists) {
			return InvitationRecipientResult{}, err
		}
		logUnexpectedProjectFailure(uc.logger, ctx, "project.invitation.accept", err)
		return InvitationRecipientResult{}, err
	}
	uc.logger.Info(ctx, "project invitation accepted", "operation", "project.invitation.accept")
	return result, nil
}

func projectInvitationTokenHash(rawToken string) ([]byte, bool) {
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(rawToken)
	if err != nil || len(decoded) != 32 {
		return nil, false
	}
	hash := sha256.Sum256(decoded)
	return hash[:], true
}

func validInvitationMemberRole(roleID string) bool {
	switch roleID {
	case "admin", "editor", "viewer":
		return true
	default:
		return false
	}
}

func invitationIssuerAuthorized(invitation dao.ProjectInvitation) bool {
	if invitation.InvitedBy == invitation.ProjectOwnerID {
		return true
	}
	return invitation.RoleID != "admin" && invitation.InviterRoleID == "admin"
}
