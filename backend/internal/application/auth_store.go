package application

import (
	"github.com/Najah7/task2todaytodo/internal/application/auth/repository"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuthStore struct {
	Users                *repository.UserRepository
	PersonalAccessTokens *repository.PersonalAccessTokenRepository
	Roles                *repository.RoleRepository
}

func newAuthStore(pool *pgxpool.Pool) AuthStore {
	return AuthStore{
		Users:                repository.NewUserRepository(pool),
		PersonalAccessTokens: repository.NewPersonalAccessTokenRepository(pool),
		Roles:                repository.NewRoleRepository(pool),
	}
}

func (s AuthStore) WithTx(tx pgx.Tx) AuthStore {
	return AuthStore{
		Users:                s.Users.WithTx(tx),
		PersonalAccessTokens: s.PersonalAccessTokens.WithTx(tx),
		Roles:                s.Roles.WithTx(tx),
	}
}
