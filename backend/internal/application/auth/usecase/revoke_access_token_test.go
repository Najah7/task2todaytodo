package usecase

import (
	"context"
	"testing"
	"time"
)

func TestRevokeAccessTokenUseCaseExecute(t *testing.T) {
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
			err := NewRevokeAccessTokenUseCase(tt.repo, nil).Execute(context.Background(), tt.token)
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
