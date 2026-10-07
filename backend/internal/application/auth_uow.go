package application

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuthUOW struct {
	pool  *pgxpool.Pool
	store AuthStore
}

func NewAuthUOW(pool *pgxpool.Pool, store AuthStore) usecase.UOW {
	return &AuthUOW{
		pool:  pool,
		store: store,
	}
}

func (u *AuthUOW) Do(
	ctx context.Context,
	fn func(ctx context.Context, repos usecase.Repositories) error,
) error {
	return RunInTx(ctx, u.pool, func(tx pgx.Tx) error {
		return fn(ctx, authRepositories{store: u.store.WithTx(tx)})
	})
}

type authRepositories struct {
	store AuthStore
}

func (r authRepositories) Users() usecase.UserRepository {
	return r.store.Users
}

func (r authRepositories) PersonalAccessTokens() usecase.PersonalAccessTokenRepository {
	return r.store.PersonalAccessTokens
}

func (r authRepositories) Roles() usecase.RoleCatalogRepository {
	return r.store.Roles
}
