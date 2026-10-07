package dao

type PersonalAccessToken struct {
	Token     string
	UserID    string
	ExpiresAt int64
	RevokedAt int64
	CreatedAt int64
}
