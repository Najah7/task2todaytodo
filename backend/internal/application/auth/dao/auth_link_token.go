package dao

import "time"

type AuthLinkToken struct {
	TokenHash []byte
	UserID    string
	Purpose   string
	ExpiresAt time.Time
	UsedAt    time.Time
	RevokedAt time.Time
}
