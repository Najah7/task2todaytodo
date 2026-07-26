package usecase

import (
	"context"

	domain "github.com/Najah7/task2todaytodo/internal/auth/domain"
)

type UserRepository interface {
	Get(ctx context.Context, id domain.UserID) (domain.User, error)
	GetByEmail(ctx context.Context, email string) (domain.User, error)
	Create(ctx context.Context, user domain.User) (domain.User, error)
	Update(ctx context.Context, user domain.User) (domain.User, error)
}

type AccessTokenRepository interface {
	GetByToken(ctx context.Context, token string) (domain.AccessToken, error)
	Create(ctx context.Context, token domain.AccessToken) (domain.AccessToken, error)
	Revoke(ctx context.Context, token string) error
}
