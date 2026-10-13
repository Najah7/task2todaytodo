package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/auth/dao"
	domain "github.com/Najah7/task2todaytodo/internal/application/auth/domain"
)

var (
	errGetUser                   = errors.New("get user failed")
	errGetUserByEmail            = errors.New("get user by email failed")
	errUpdateUser                = errors.New("update user failed")
	errGetPersonalAccessToken    = errors.New("get access token failed")
	errRevokePersonalAccessToken = errors.New("revoke access token failed")
)

var _ UserRepository = (*stubUserRepository)(nil)

type stubUserRepository struct {
	user              domain.User
	userByEmail       domain.User
	updateResult      domain.User
	getErr            error
	getByEmailErr     error
	updateErr         error
	updateTimezoneErr error
}

func (r *stubUserRepository) Get(_ context.Context, _ domain.UserID) (dao.User, error) {
	if r.getErr != nil {
		return dao.User{}, r.getErr
	}
	return userDAOFromDomain(r.user), nil
}

func (r *stubUserRepository) GetByEmail(_ context.Context, _ string) (dao.User, error) {
	if r.getByEmailErr != nil {
		return dao.User{}, r.getByEmailErr
	}
	return userDAOFromDomain(r.userByEmail), nil
}

func (r *stubUserRepository) UpdateName(_ context.Context, id domain.UserID, name domain.UserName) (dao.User, error) {
	if r.updateErr != nil {
		return dao.User{}, r.updateErr
	}
	user := r.user
	if !r.updateResult.IsZero() {
		user = r.updateResult
	}
	user.ID, user.Name = id, name
	return userDAOFromDomain(user), nil
}

func (r *stubUserRepository) UpdateEmail(_ context.Context, id domain.UserID, email domain.Email) (dao.User, error) {
	if r.updateErr != nil {
		return dao.User{}, r.updateErr
	}
	user := r.user
	if !r.updateResult.IsZero() {
		user = r.updateResult
	}
	user.ID, user.Email = id, email
	return userDAOFromDomain(user), nil
}

func (r *stubUserRepository) UpdateTimezone(_ context.Context, _ domain.UserID, _ domain.UserTimezone) error {
	return r.updateTimezoneErr
}

func (*stubUserRepository) UpdateActiveTimeSettings(_ context.Context, _ domain.UserID, _, _ string) error {
	return nil
}

func (*stubUserRepository) UpdateMinimumGapMinutes(_ context.Context, _ domain.UserID, _ int) error {
	return nil
}

func (*stubUserRepository) TimezoneExists(_ context.Context, timezone string) (bool, error) {
	switch timezone {
	case "UTC", "Asia/Tokyo", "America/Los_Angeles":
		return true, nil
	default:
		return false, nil
	}
}

var _ PersonalAccessTokenRepository = (*stubPersonalAccessTokenRepository)(nil)

type stubPersonalAccessTokenRepository struct {
	token        domain.PersonalAccessToken
	getErr       error
	revokeErr    error
	revokedToken string
}

func (r *stubPersonalAccessTokenRepository) GetByToken(_ context.Context, _ string) (dao.PersonalAccessToken, error) {
	if r.getErr != nil {
		return dao.PersonalAccessToken{}, r.getErr
	}
	return personalAccessTokenDAOFromDomain(r.token), nil
}

func (r *stubPersonalAccessTokenRepository) CreateForAuth(_ context.Context, token domain.PersonalAccessToken, _ int64) (dao.PersonalAccessToken, error) {
	return personalAccessTokenDAOFromDomain(token), nil
}

func (r *stubPersonalAccessTokenRepository) Revoke(_ context.Context, token string) error {
	if r.revokeErr != nil {
		return r.revokeErr
	}
	r.revokedToken = token
	return nil
}

func existingUser(t *testing.T) domain.User {
	t.Helper()
	userID := domain.UserID("user-1")
	email, err := domain.NewEmail("user@example.com")
	if err != nil {
		t.Fatalf("domain.NewEmail() error = %v", err)
	}
	password, err := domain.NewPassword("Password1!")
	if err != nil {
		t.Fatalf("domain.NewPassword() error = %v", err)
	}
	return domain.NewUser(userID, email, password, domain.NewUserName("John", "Doe"))
}

func servicePersonalAccessToken(t *testing.T) domain.PersonalAccessToken {
	t.Helper()
	token, err := domain.NewExistingPersonalAccessToken("token-1", "user-1", time.Now().Add(time.Hour).Unix(), 0, time.Now().Add(-time.Hour).Unix())
	if err != nil {
		t.Fatalf("domain.NewExistingPersonalAccessToken() error = %v", err)
	}
	return token
}

func assertUserEqual(t *testing.T, got, want domain.User) {
	t.Helper()
	if got != want {
		t.Errorf("user = %+v, want %+v", got, want)
	}
}

func userDAOFromDomain(user domain.User) dao.User {
	return dao.User{
		ID:              string(user.ID),
		FirstName:       user.Name.FirstName,
		LastName:        user.Name.LastName,
		Email:           user.Email.String(),
		PasswordHash:    user.Password.String(),
		EmailVerifiedAt: 1,
		Timezone:        user.Timezone.String(),
	}
}

func userReadDAOFromDomain(user domain.User) dao.User {
	result := userDAOFromDomain(user)
	result.PasswordHash = ""
	return result
}

func personalAccessTokenDAOFromDomain(token domain.PersonalAccessToken) dao.PersonalAccessToken {
	return dao.PersonalAccessToken{
		Token:     token.Token,
		UserID:    string(token.UserID),
		ExpiresAt: token.ExpiresAt,
		RevokedAt: token.RevokedAt,
		CreatedAt: token.CreatedAt,
	}
}

func assertServiceErrorIs(t *testing.T, got, want error) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Errorf("error = %v, want %v", got, want)
	}
}

func assertPersonalAccessTokenErrorIs(t *testing.T, got, want error) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Errorf("error = %v, want %v", got, want)
	}
}

type passwordUpdateUOW struct{ repository *passwordUpdateRepository }

func (u passwordUpdateUOW) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	return fn(ctx, passwordUpdateRepositories{repository: u.repository})
}

type passwordUpdateRepositories struct{ repository *passwordUpdateRepository }

func (passwordUpdateRepositories) Users() UserRepository                               { return nil }
func (passwordUpdateRepositories) Availability() UserAvailabilityValidator             { return nil }
func (passwordUpdateRepositories) PersonalAccessTokens() PersonalAccessTokenRepository { return nil }
func (passwordUpdateRepositories) Roles() RoleCatalogRepository                        { return nil }
func (r passwordUpdateRepositories) EmailAuth() EmailAuthRepository                    { return r.repository }

type passwordUpdateRepository struct {
	EmailAuthRepository
	user dao.User
	err  error
}

func (r *passwordUpdateRepository) GetUserByIDForAuth(context.Context, string) (dao.User, error) {
	return r.user, r.err
}
func (r *passwordUpdateRepository) UpdateUserPasswordForReset(_ context.Context, _ string, hash string) error {
	r.user.PasswordHash = hash
	return r.err
}
func (*passwordUpdateRepository) RevokeAllUserPersonalAccessTokens(context.Context, string) error {
	return nil
}
func (*passwordUpdateRepository) RevokeUserAuthLinkTokens(context.Context, string) error { return nil }
