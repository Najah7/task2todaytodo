package usecase

import "context"

type UnitOfWork interface {
	Do(ctx context.Context, fn func(ctx context.Context, repos Repositories) error) error
}

type Repositories interface {
	Users() UserRepository
	AccessTokens() AccessTokenRepository
}
