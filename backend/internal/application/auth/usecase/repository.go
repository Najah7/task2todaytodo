package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/auth/dao"
	domain "github.com/Najah7/task2todaytodo/internal/application/auth/domain"
)

type UserRepository interface {
	Get(ctx context.Context, id domain.UserID) (dao.User, error)
	GetByEmail(ctx context.Context, email string) (dao.User, error)
	Create(ctx context.Context, user domain.User) (dao.User, error)
	Update(ctx context.Context, user domain.User) (dao.User, error)
	UpdateTimezone(ctx context.Context, id domain.UserID, timezone domain.UserTimezone) error
}

type PersonalAccessTokenRepository interface {
	GetByToken(ctx context.Context, token string) (dao.PersonalAccessToken, error)
	Create(ctx context.Context, token domain.PersonalAccessToken) (dao.PersonalAccessToken, error)
	Revoke(ctx context.Context, token string) error
}
