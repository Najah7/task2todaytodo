package application

import (
	"context"

	authusecase "github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuthUOW struct {
	pool                *pgxpool.Pool
	store               AuthStore
	availabilityFactory func(pgx.Tx) authusecase.UserAvailabilityValidator
}

func NewAuthUOW(pool *pgxpool.Pool, store AuthStore, availabilityFactory ...func(pgx.Tx) authusecase.UserAvailabilityValidator) authusecase.UOW {
	var bind func(pgx.Tx) authusecase.UserAvailabilityValidator
	if len(availabilityFactory) > 0 {
		bind = availabilityFactory[0]
	}
	return &AuthUOW{
		pool: pool, store: store, availabilityFactory: bind,
	}
}

func (u *AuthUOW) Do(
	ctx context.Context,
	fn func(ctx context.Context, repos authusecase.Repositories) error,
) error {
	return RunInTodoWorkflowTx(ctx, u.pool, func(tx pgx.Tx) error {
		var availability authusecase.UserAvailabilityValidator
		if u.availabilityFactory != nil {
			availability = u.availabilityFactory(tx)
		}
		return fn(ctx, authRepositories{store: u.store.WithTx(tx), availability: availability})
	})
}

type authRepositories struct {
	store        AuthStore
	availability authusecase.UserAvailabilityValidator
}

func (r authRepositories) Users() authusecase.UserRepository {
	return r.store.Users
}

func (r authRepositories) Availability() authusecase.UserAvailabilityValidator { return r.availability }

func (r authRepositories) PersonalAccessTokens() authusecase.PersonalAccessTokenRepository {
	return r.store.PersonalAccessTokens
}

func (r authRepositories) Roles() authusecase.RoleCatalogRepository {
	return r.store.Roles
}
