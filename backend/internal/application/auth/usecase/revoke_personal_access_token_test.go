package usecase

import (
	"context"
	"testing"
	"time"
)

func TestRevokePersonalAccessTokenUseCaseExecute(t *testing.T) {
	activeToken := servicePersonalAccessToken(t)
	revokedToken := servicePersonalAccessToken(t)
	revokedToken.RevokedAt = time.Now().Unix()

	tests := []struct {
		name        string
		token       string
		repo        *stubPersonalAccessTokenRepository
		wantErr     error
		wantRevoked bool
	}{
		{name: "success", token: "token-1", repo: &stubPersonalAccessTokenRepository{token: activeToken}, wantRevoked: true},
		{name: "get error", token: "token-1", repo: &stubPersonalAccessTokenRepository{getErr: errGetPersonalAccessToken}, wantErr: errGetPersonalAccessToken},
		{name: "already revoked", token: "token-1", repo: &stubPersonalAccessTokenRepository{token: revokedToken}},
		{name: "revoke error", token: "token-1", repo: &stubPersonalAccessTokenRepository{token: activeToken, revokeErr: errRevokePersonalAccessToken}, wantErr: errRevokePersonalAccessToken},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewRevokePersonalAccessTokenUseCase(tt.repo, nil).Execute(context.Background(), tt.token)
			assertPersonalAccessTokenErrorIs(t, err, tt.wantErr)
			if tt.wantRevoked && tt.repo.revokedToken != "token-1" {
				t.Errorf("revoked token = %q, want %q", tt.repo.revokedToken, "token-1")
			}
			if !tt.wantRevoked && tt.repo.revokedToken != "" {
				t.Errorf("revoked token = %q, want no revoke call", tt.repo.revokedToken)
			}
		})
	}
}
