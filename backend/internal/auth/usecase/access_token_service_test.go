package usecase

import (
	"context"
	"errors"
	domain "github.com/Najah7/task2todaytodo/internal/auth/domain"
	"testing"
	"time"
)

var (
	errGetAccessToken    = errors.New("get access token failed")
	errCreateAccessToken = errors.New("create access token failed")
	errRevokeAccessToken = errors.New("revoke access token failed")
)

func TestAccessTokenServiceGetByToken(t *testing.T) {
	want := serviceAccessToken(t)
	tests := []struct {
		name    string
		repo    *stubAccessTokenRepository
		wantErr error
	}{
		{name: "success", repo: &stubAccessTokenRepository{token: want}},
		{name: "repository error", repo: &stubAccessTokenRepository{getErr: errGetAccessToken}, wantErr: errGetAccessToken},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewAccessTokenService(tt.repo).GetByToken(context.Background(), "token-1")
			assertAccessTokenErrorIs(t, err, tt.wantErr)
			if tt.wantErr == nil && got != want {
				t.Errorf("access token = %+v, want %+v", got, want)
			}
		})
	}
}

func TestAccessTokenServiceGenerate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo := &stubAccessTokenRepository{}
		got, err := NewAccessTokenService(repo).Generate(context.Background(), "user-1")
		assertAccessTokenErrorIs(t, err, nil)
		if got.IsZero() || got.UserID != "user-1" || repo.createdToken != got {
			t.Errorf("access token = %+v, want persisted token for user-1", got)
		}
	})

	t.Run("empty user ID", func(t *testing.T) {
		got, err := NewAccessTokenService(&stubAccessTokenRepository{}).Generate(context.Background(), "")
		assertAccessTokenErrorIs(t, err, domain.ErrAccessTokenUserIDEmpty)
		if !got.IsZero() {
			t.Errorf("access token = %+v, want zero access token", got)
		}
	})

	t.Run("repository error", func(t *testing.T) {
		_, err := NewAccessTokenService(&stubAccessTokenRepository{createErr: errCreateAccessToken}).Generate(context.Background(), "user-1")
		assertAccessTokenErrorIs(t, err, errCreateAccessToken)
	})
}

func TestAccessTokenServiceRevoke(t *testing.T) {
	activeToken := serviceAccessToken(t)
	revokedToken := serviceAccessToken(t)
	revokedToken.RevokedAt = time.Now().Unix()

	tests := []struct {
		name        string
		token       string
		repo        *stubAccessTokenRepository
		wantErr     error
		wantRevoked bool
	}{
		{name: "success", token: "token-1", repo: &stubAccessTokenRepository{token: activeToken}, wantRevoked: true},
		{name: "get error", token: "token-1", repo: &stubAccessTokenRepository{getErr: errGetAccessToken}, wantErr: errGetAccessToken},
		{name: "already revoked", token: "token-1", repo: &stubAccessTokenRepository{token: revokedToken}},
		{name: "revoke error", token: "token-1", repo: &stubAccessTokenRepository{token: activeToken, revokeErr: errRevokeAccessToken}, wantErr: errRevokeAccessToken},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewAccessTokenService(tt.repo).Revoke(context.Background(), tt.token)
			assertAccessTokenErrorIs(t, err, tt.wantErr)
			if tt.wantRevoked && tt.repo.revokedToken != "token-1" {
				t.Errorf("revoked token = %q, want %q", tt.repo.revokedToken, "token-1")
			}
			if !tt.wantRevoked && tt.repo.revokedToken != "" {
				t.Errorf("revoked token = %q, want no revoke call", tt.repo.revokedToken)
			}
		})
	}
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

func (r *stubAccessTokenRepository) GetByToken(_ context.Context, _ string) (domain.AccessToken, error) {
	if r.getErr != nil {
		return domain.NewZeroAccessToken(), r.getErr
	}
	return r.token, nil
}

func (r *stubAccessTokenRepository) Create(_ context.Context, token domain.AccessToken) (domain.AccessToken, error) {
	if r.createErr != nil {
		return domain.NewZeroAccessToken(), r.createErr
	}
	r.createdToken = token
	return token, nil
}

func (r *stubAccessTokenRepository) Revoke(_ context.Context, token string) error {
	if r.revokeErr != nil {
		return r.revokeErr
	}
	r.revokedToken = token
	return nil
}

func serviceAccessToken(t *testing.T) domain.AccessToken {
	t.Helper()
	token, err := domain.NewExistingAccessToken("token-1", "user-1", time.Now().Add(time.Hour).Unix(), 0, time.Now().Add(-time.Hour).Unix())
	if err != nil {
		t.Fatalf("domain.NewExistingAccessToken() error = %v", err)
	}
	return token
}

func assertAccessTokenErrorIs(t *testing.T, got, want error) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Errorf("error = %v, want %v", got, want)
	}
}
