package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/auth/dao"
	"github.com/Najah7/task2todaytodo/internal/application/auth/domain"
	"github.com/jackc/pgx/v5"
)

func TestAcceptInvitationRecipientRequiresVerifiedAccountSession(t *testing.T) {
	user := invitationTestUser(t, true)
	repositories := invitationTestRepositories{email: &invitationTestEmailAuth{user: user}}

	_, err := AcceptInvitationRecipient(context.Background(), repositories, "new-user", user.Email, "", "")
	if !errors.Is(err, ErrInvitationRecipientAuthenticationRequired) {
		t.Fatalf("AcceptInvitationRecipient() error = %v, want authentication required", err)
	}
}

func TestAcceptInvitationRecipientRejectsAuthenticatedAccountMismatch(t *testing.T) {
	user := invitationTestUser(t, true)
	repositories := invitationTestRepositories{email: &invitationTestEmailAuth{user: user}}

	_, err := AcceptInvitationRecipient(context.Background(), repositories, "new-user", user.Email, "", "another-user")
	if !errors.Is(err, ErrInvitationRecipientAuthenticationRequired) {
		t.Fatalf("AcceptInvitationRecipient() error = %v, want authentication required", err)
	}
}

func TestAcceptInvitationRecipientReusesUnverifiedAccountAndRevokesSessions(t *testing.T) {
	user := invitationTestUser(t, false)
	email := &invitationTestEmailAuth{user: user}
	pat := &invitationTestPAT{}
	repositories := invitationTestRepositories{email: email, pat: pat}

	accepted, err := AcceptInvitationRecipient(context.Background(), repositories, "new-user", user.Email, "NewPassword1!", "")
	if err != nil {
		t.Fatalf("AcceptInvitationRecipient() error = %v", err)
	}
	if accepted.User.ID != user.ID || !accepted.NeedsAutoLogin || accepted.PersonalAccessToken.Token == "" {
		t.Fatalf("AcceptInvitationRecipient() result = %+v", accepted)
	}
	if email.updatedPassword == "" || !email.verified || !email.revokedTokens || !email.revokedLinks {
		t.Fatalf("email auth changes = %+v, want password, verified account, and revoked sessions and links", email)
	}
	if pat.createdAuthVersion != user.AuthVersion+1 {
		t.Fatalf("PAT auth version = %d, want %d", pat.createdAuthVersion, user.AuthVersion+1)
	}
}

func TestAcceptInvitationRecipientCreatesVerifiedAccount(t *testing.T) {
	email := &invitationTestEmailAuth{getErr: pgx.ErrNoRows}
	pat := &invitationTestPAT{}
	repositories := invitationTestRepositories{email: email, pat: pat}

	accepted, err := AcceptInvitationRecipient(context.Background(), repositories, "new-user", "new@example.com", "NewPassword1!", "")
	if err != nil {
		t.Fatalf("AcceptInvitationRecipient() error = %v", err)
	}
	if accepted.User.ID != "new-user" || accepted.User.Email != "new@example.com" || accepted.User.EmailVerifiedAt == 0 || !accepted.NeedsAutoLogin {
		t.Fatalf("AcceptInvitationRecipient() result = %+v, want verified new account", accepted)
	}
	if email.createdID != "new-user" || !email.verified || accepted.PersonalAccessToken.Token == "" {
		t.Fatalf("created account = %+v, result = %+v", email, accepted)
	}
}

func invitationTestUser(t *testing.T, verified bool) dao.User {
	t.Helper()
	user := dao.User{ID: "user-1", Email: "user@example.com", PasswordHash: "old-hash", AuthVersion: 2}
	if verified {
		user.EmailVerifiedAt = 1
	}
	return user
}

type invitationTestRepositories struct {
	Repositories
	email *invitationTestEmailAuth
	pat   *invitationTestPAT
}

func (r invitationTestRepositories) EmailAuth() EmailAuthRepository { return r.email }
func (r invitationTestRepositories) PersonalAccessTokens() PersonalAccessTokenRepository {
	return r.pat
}

type invitationTestEmailAuth struct {
	EmailAuthRepository
	user            dao.User
	getErr          error
	createdID       string
	updatedPassword string
	verified        bool
	revokedTokens   bool
	revokedLinks    bool
}

func (r *invitationTestEmailAuth) GetUserByEmailForAuth(context.Context, string) (dao.User, error) {
	return r.user, r.getErr
}

func (r *invitationTestEmailAuth) LockEmailDeliveryAttempts(context.Context, []byte) error {
	return nil
}

func (r *invitationTestEmailAuth) CreateUnverifiedUser(_ context.Context, id, email, passwordHash string) (dao.User, error) {
	r.createdID = id
	r.user = dao.User{ID: id, Email: email, PasswordHash: passwordHash, AuthVersion: 1}
	return r.user, nil
}

func (r *invitationTestEmailAuth) MarkUserEmailVerified(context.Context, string) error {
	r.verified = true
	return nil
}

func (r *invitationTestEmailAuth) UpdateUserPasswordForReset(_ context.Context, userID, passwordHash string) error {
	r.updatedPassword = passwordHash
	r.user.ID = userID
	r.user.PasswordHash = passwordHash
	return nil
}

func (r *invitationTestEmailAuth) RevokeAllUserPersonalAccessTokens(context.Context, string) error {
	r.revokedTokens = true
	return nil
}

func (r *invitationTestEmailAuth) RevokeUserAuthLinkTokens(context.Context, string) error {
	r.revokedLinks = true
	return nil
}

type invitationTestPAT struct {
	PersonalAccessTokenRepository
	createdAuthVersion int64
}

func (r *invitationTestPAT) CreateForAuth(_ context.Context, token domain.PersonalAccessToken, authVersion int64) (dao.PersonalAccessToken, error) {
	r.createdAuthVersion = authVersion
	return dao.PersonalAccessToken{Token: token.Token, UserID: string(token.UserID), ExpiresAt: token.ExpiresAt}, nil
}
