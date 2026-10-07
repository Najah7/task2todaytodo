package usecase

import "context"

type UOW interface {
	Do(ctx context.Context, fn func(ctx context.Context, repos Repositories) error) error
}

type Repositories interface {
	Users() UserRepository
	PersonalAccessTokens() PersonalAccessTokenRepository
	Roles() RoleCatalogRepository
}
