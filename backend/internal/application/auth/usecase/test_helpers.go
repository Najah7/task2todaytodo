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
	errGetUser           = errors.New("get user failed")
	errGetUserByEmail    = errors.New("get user by email failed")
	errCreateUser        = errors.New("create user failed")
	errUpdateUser        = errors.New("update user failed")
	errGetAccessToken    = errors.New("get access token failed")
	errCreateAccessToken = errors.New("create access token failed")
	errRevokeAccessToken = errors.New("revoke access token failed")
)

var _ UserRepository = (*stubUserRepository)(nil)

type stubUserRepository struct {
	user              domain.User
	userByEmail       domain.User
	createdUser       domain.User
	updateResult      domain.User
	getErr            error
	getByEmailErr     error
	createErr         error
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

func (r *stubUserRepository) Create(_ context.Context, user domain.User) (dao.User, error) {
	if r.createErr != nil {
		return dao.User{}, r.createErr
	}
	r.createdUser = user
	return userDAOFromDomain(user), nil
}

func (r *stubUserRepository) Update(_ context.Context, user domain.User) (dao.User, error) {
	if r.updateErr != nil {
		return dao.User{}, r.updateErr
	}
	if !r.updateResult.IsZero() {
		return userDAOFromDomain(r.updateResult), nil
	}
	return userDAOFromDomain(user), nil
}

func (r *stubUserRepository) UpdateTimezone(_ context.Context, _ domain.UserID, _ domain.UserTimezone) error {
	return r.updateTimezoneErr
}

var _ AccessTokenRepository = (*stubAccessTokenRepository)(nil)

type stubAccessTokenRepository struct {
	token        domain.AccessToken
	getErr       error
	createErr    error
	revokeErr    error
	createdToken domain.AccessToken
	revokedToken string
}

func (r *stubAccessTokenRepository) GetByToken(_ context.Context, _ string) (dao.AccessToken, error) {
	if r.getErr != nil {
		return dao.AccessToken{}, r.getErr
	}
	return accessTokenDAOFromDomain(r.token), nil
}

func (r *stubAccessTokenRepository) Create(_ context.Context, token domain.AccessToken) (dao.AccessToken, error) {
	if r.createErr != nil {
		return dao.AccessToken{}, r.createErr
	}
	r.createdToken = token
	return accessTokenDAOFromDomain(token), nil
}

func (r *stubAccessTokenRepository) Revoke(_ context.Context, token string) error {
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

func serviceAccessToken(t *testing.T) domain.AccessToken {
	t.Helper()
	token, err := domain.NewExistingAccessToken("token-1", "user-1", time.Now().Add(time.Hour).Unix(), 0, time.Now().Add(-time.Hour).Unix())
	if err != nil {
		t.Fatalf("domain.NewExistingAccessToken() error = %v", err)
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
		ID:        string(user.ID),
		FirstName: user.Name.FirstName,
		LastName:  user.Name.LastName,
		Email:     user.Email.String(),
		Password:  user.Password.String(),
		Timezone:  user.Timezone.String(),
	}
}

func userReadDAOFromDomain(user domain.User) dao.User {
	result := userDAOFromDomain(user)
	result.Password = ""
	return result
}

func accessTokenDAOFromDomain(token domain.AccessToken) dao.AccessToken {
	return dao.AccessToken{
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

func assertAccessTokenErrorIs(t *testing.T, got, want error) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Errorf("error = %v, want %v", got, want)
	}
}
