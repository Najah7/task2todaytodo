package dao

type AccessToken struct {
	Token     string
	UserID    string
	ExpiresAt int64
	RevokedAt int64
	CreatedAt int64
}
