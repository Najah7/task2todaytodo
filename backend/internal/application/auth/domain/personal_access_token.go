package domain

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"
)

const (
	ActivePersonalAccessTokenDurationDays = 7 * 24 * time.Hour // 7 days
)

var (
	ErrPersonalAccessTokenEmpty            = errors.New("access token cannot be empty")
	ErrPersonalAccessTokenUserIDEmpty      = errors.New("access token user ID cannot be empty")
	ErrPersonalAccessTokenExpiresAtInvalid = errors.New("access token expiration must be set")
	ErrPersonalAccessTokenRevoked          = errors.New("access token is already revoked")
	ErrPersonalAccessTokenExpired          = errors.New("access token is already expired")
	ErrPersonalAccessTokenUserMismatch     = errors.New("access token does not belong to user")
)

func generateToken() (string, error) {
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(token), nil
}

type PersonalAccessToken struct {
	Token     string
	UserID    UserID
	ExpiresAt int64
	RevokedAt int64
	CreatedAt int64
}

func NewPersonalAccessToken(userID UserID) (PersonalAccessToken, error) {
	token, err := generateToken()
	if err != nil {
		return NewZeroPersonalAccessToken(), err
	}
	expiresAt := time.Now().Add(ActivePersonalAccessTokenDurationDays).Unix()

	personalAccessToken := PersonalAccessToken{
		Token:     token,
		UserID:    userID,
		ExpiresAt: expiresAt,
		RevokedAt: 0,
		CreatedAt: time.Now().Unix(),
	}
	if err := personalAccessToken.Validate(); err != nil {
		return NewZeroPersonalAccessToken(), err
	}

	return personalAccessToken, nil
}

func NewExistingPersonalAccessToken(token string, userID UserID, expiresAt, revokedAt, createdAt int64) (PersonalAccessToken, error) {
	personalAccessToken := PersonalAccessToken{
		Token:     token,
		UserID:    userID,
		ExpiresAt: expiresAt,
		RevokedAt: revokedAt,
		CreatedAt: createdAt,
	}
	if err := personalAccessToken.Validate(); err != nil {
		return NewZeroPersonalAccessToken(), err
	}

	return personalAccessToken, nil
}

func NewZeroPersonalAccessToken() PersonalAccessToken {
	return PersonalAccessToken{}
}

func (at PersonalAccessToken) IsZero() bool {
	return at.Token == ""
}

func (at PersonalAccessToken) IsRevoked() bool {
	return at.RevokedAt != 0
}

func (at PersonalAccessToken) IsExpired() bool {
	return at.IsExpiredAt(time.Now())
}

func (at PersonalAccessToken) IsExpiredAt(t time.Time) bool {
	return at.ExpiresAt <= t.Unix()
}

func (at PersonalAccessToken) Validate() error {
	if at.Token == "" {
		return ErrPersonalAccessTokenEmpty
	}
	if at.UserID == "" {
		return ErrPersonalAccessTokenUserIDEmpty
	}
	if at.ExpiresAt <= 0 {
		return ErrPersonalAccessTokenExpiresAtInvalid
	}

	if at.IsRevoked() {
		return ErrPersonalAccessTokenRevoked
	}
	if at.IsExpired() {
		return ErrPersonalAccessTokenExpired
	}

	return nil
}
