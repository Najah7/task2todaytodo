package usecase

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"

	"github.com/Najah7/task2todaytodo/internal/application/auth/dao"
	"github.com/Najah7/task2todaytodo/internal/application/auth/domain"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrInvitationRecipientAuthenticationRequired = errors.New("invitation recipient must authenticate with the invited account")

type InvitationRecipientResult struct {
	User                dao.User
	PersonalAccessToken dao.PersonalAccessToken
	NeedsAutoLogin      bool
}

// AcceptInvitationRecipient applies the auth-side part of project invitation
// acceptance using repositories already bound to the caller's transaction.
func AcceptInvitationRecipient(
	ctx context.Context,
	repositories Repositories,
	newUserID string,
	email, password, authenticatedUserID string,
) (InvitationRecipientResult, error) {
	address, err := domain.NewEmail(email)
	if err != nil {
		return InvitationRecipientResult{}, ErrInvalidEmail
	}
	auth := repositories.EmailAuth()
	addressHash := sha256.Sum256([]byte(strings.ToLower(address.String())))
	if err := auth.LockEmailDeliveryAttempts(ctx, addressHash[:]); err != nil {
		return InvitationRecipientResult{}, err
	}
	user, err := auth.GetUserByEmailForAuth(ctx, address.String())
	if err != nil {
		if !telemetry.IsNotFound(err) {
			return InvitationRecipientResult{}, err
		}
		if authenticatedUserID != "" || password == "" || newUserID == "" {
			return InvitationRecipientResult{}, ErrInvitationRecipientAuthenticationRequired
		}
		return createInvitationRecipient(ctx, repositories, auth, newUserID, address, password)
	}
	if authenticatedUserID != "" && authenticatedUserID != user.ID {
		return InvitationRecipientResult{}, ErrInvitationRecipientAuthenticationRequired
	}
	if user.EmailVerifiedAt != 0 {
		if authenticatedUserID != user.ID {
			return InvitationRecipientResult{}, ErrInvitationRecipientAuthenticationRequired
		}
		return InvitationRecipientResult{User: user}, nil
	}
	if password == "" {
		return InvitationRecipientResult{}, domain.ErrPasswordEmpty
	}
	newPassword, err := domain.NewPassword(password)
	if err != nil {
		return InvitationRecipientResult{}, err
	}
	if err := auth.UpdateUserPasswordForReset(ctx, user.ID, newPassword.String()); err != nil {
		return InvitationRecipientResult{}, err
	}
	if err := auth.RevokeAllUserPersonalAccessTokens(ctx, user.ID); err != nil {
		return InvitationRecipientResult{}, err
	}
	if err := auth.RevokeUserAuthLinkTokens(ctx, user.ID); err != nil {
		return InvitationRecipientResult{}, err
	}
	user.PasswordHash = newPassword.String()
	user.EmailVerifiedAt = 1
	user.AuthVersion++
	return issueInvitationRecipientToken(ctx, repositories, user)
}

func createInvitationRecipient(
	ctx context.Context,
	repositories Repositories,
	auth EmailAuthRepository,
	userID string,
	email domain.Email,
	password string,
) (InvitationRecipientResult, error) {
	newPassword, err := domain.NewPassword(password)
	if err != nil {
		return InvitationRecipientResult{}, err
	}
	user, err := auth.CreateUnverifiedUser(ctx, userID, email.String(), newPassword.String())
	if err != nil {
		var databaseError *pgconn.PgError
		if errors.As(err, &databaseError) && databaseError.Code == "23505" && databaseError.ConstraintName == "users_email_key" {
			return InvitationRecipientResult{}, ErrUserEmailAlreadyExists
		}
		return InvitationRecipientResult{}, err
	}
	if err := auth.MarkUserEmailVerified(ctx, user.ID); err != nil {
		return InvitationRecipientResult{}, err
	}
	user.PasswordHash = newPassword.String()
	user.EmailVerifiedAt = 1
	return issueInvitationRecipientToken(ctx, repositories, user)
}

func issueInvitationRecipientToken(ctx context.Context, repositories Repositories, user dao.User) (InvitationRecipientResult, error) {
	token, err := domain.NewPersonalAccessToken(domain.UserID(user.ID))
	if err != nil {
		return InvitationRecipientResult{}, err
	}
	issued, err := repositories.PersonalAccessTokens().CreateForAuth(ctx, token, user.AuthVersion)
	if err != nil {
		return InvitationRecipientResult{}, err
	}
	return InvitationRecipientResult{User: user, PersonalAccessToken: issued, NeedsAutoLogin: true}, nil
}
