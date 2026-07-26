package domain

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"
)

const (
	ActiveAccessTokenDurationDays = 7 * 24 * time.Hour // 7 days
)

var (
	ErrAccessTokenEmpty            = errors.New("access token cannot be empty")
	ErrAccessTokenUserIDEmpty      = errors.New("access token user ID cannot be empty")
	ErrAccessTokenExpiresAtInvalid = errors.New("access token expiration must be set")
	ErrAccessTokenRevoked          = errors.New("access token is already revoked")
	ErrAccessTokenExpired          = errors.New("access token is already expired")
	ErrAccessTokenUserMismatch     = errors.New("access token does not belong to user")
)

func generateToken() (string, error) {
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(token), nil
}

type AccessToken struct {
	Token     string
	UserID    UserID
	ExpiresAt int64
	RevokedAt int64
	CreatedAt int64
}

func NewAccessToken(userID UserID) (AccessToken, error) {
	token, err := generateToken()
	if err != nil {
		return NewZeroAccessToken(), err
	}
	expiresAt := time.Now().Add(ActiveAccessTokenDurationDays).Unix()

	accessToken := AccessToken{
		Token:     token,
		UserID:    userID,
		ExpiresAt: expiresAt,
		RevokedAt: 0,
		CreatedAt: time.Now().Unix(),
	}
	if err := accessToken.Validate(); err != nil {
		return NewZeroAccessToken(), err
	}

	return accessToken, nil
}

func NewExistingAccessToken(token string, userID UserID, expiresAt, revokedAt, createdAt int64) (AccessToken, error) {
	accessToken := AccessToken{
		Token:     token,
		UserID:    userID,
		ExpiresAt: expiresAt,
		RevokedAt: revokedAt,
		CreatedAt: createdAt,
	}
	if err := accessToken.Validate(); err != nil {
		return NewZeroAccessToken(), err
	}

	return accessToken, nil
}

func NewZeroAccessToken() AccessToken {
	return AccessToken{}
}

func (at AccessToken) IsZero() bool {
	return at.Token == ""
}

func (at AccessToken) IsRevoked() bool {
	return at.RevokedAt != 0
}

func (at AccessToken) IsExpired() bool {
	return at.IsExpiredAt(time.Now())
}

func (at AccessToken) IsExpiredAt(t time.Time) bool {
	return at.ExpiresAt <= t.Unix()
}

func (at AccessToken) Validate() error {
	if at.Token == "" {
		return ErrAccessTokenEmpty
	}
	if at.UserID == "" {
		return ErrAccessTokenUserIDEmpty
	}
	if at.ExpiresAt <= 0 {
		return ErrAccessTokenExpiresAtInvalid
	}

	if at.IsRevoked() {
		return ErrAccessTokenRevoked
	}
	if at.IsExpired() {
		return ErrAccessTokenExpired
	}

	return nil
}
