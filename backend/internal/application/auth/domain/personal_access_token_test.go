package domain

import (
	"errors"
	"testing"
	"time"
)

func TestNewPersonalAccessToken(t *testing.T) {
	before := time.Now()
	token, err := NewPersonalAccessToken("user-1")
	if err != nil {
		t.Fatalf("NewPersonalAccessToken() error = %v", err)
	}

	if token.Token == "" || token.UserID != "user-1" {
		t.Errorf("access token = %+v, want generated token for user-1", token)
	}
	if token.RevokedAt != 0 {
		t.Errorf("RevokedAt = %d, want 0", token.RevokedAt)
	}
	if token.CreatedAt < before.Unix() || token.ExpiresAt <= token.CreatedAt {
		t.Errorf("access token timestamps = %+v, want a newly created active token", token)
	}
}

func TestNewPersonalAccessTokenRequiresUserID(t *testing.T) {
	token, err := NewPersonalAccessToken("")
	assertPersonalAccessTokenErrorIs(t, err, ErrPersonalAccessTokenUserIDEmpty)
	if !token.IsZero() {
		t.Errorf("access token = %+v, want zero access token", token)
	}
}

func TestNewExistingPersonalAccessToken(t *testing.T) {
	now := time.Now().Unix()
	tests := []struct {
		name    string
		token   string
		userID  UserID
		expires int64
		revoked int64
		wantErr error
	}{
		{name: "active", token: "token-1", userID: "user-1", expires: now + 60},
		{name: "empty token", userID: "user-1", expires: now + 60, wantErr: ErrPersonalAccessTokenEmpty},
		{name: "empty user ID", token: "token-1", expires: now + 60, wantErr: ErrPersonalAccessTokenUserIDEmpty},
		{name: "missing expiration", token: "token-1", userID: "user-1", wantErr: ErrPersonalAccessTokenExpiresAtInvalid},
		{name: "revoked", token: "token-1", userID: "user-1", expires: now + 60, revoked: now, wantErr: ErrPersonalAccessTokenRevoked},
		{name: "expired", token: "token-1", userID: "user-1", expires: now - 60, wantErr: ErrPersonalAccessTokenExpired},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewExistingPersonalAccessToken(tt.token, tt.userID, tt.expires, tt.revoked, now-1)
			assertPersonalAccessTokenErrorIs(t, err, tt.wantErr)
			if tt.wantErr != nil && !got.IsZero() {
				t.Errorf("access token = %+v, want zero access token", got)
			}
		})
	}
}

func TestPersonalAccessTokenIsExpiredAt(t *testing.T) {
	token := PersonalAccessToken{ExpiresAt: 100}
	if !token.IsExpiredAt(time.Unix(100, 0)) {
		t.Error("IsExpiredAt() = false at the expiration time, want true")
	}
	if token.IsExpiredAt(time.Unix(99, 0)) {
		t.Error("IsExpiredAt() = true before expiration, want false")
	}
}

func assertPersonalAccessTokenErrorIs(t *testing.T, got, want error) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Errorf("error = %v, want %v", got, want)
	}
}
