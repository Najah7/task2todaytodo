package application

import "github.com/jackc/pgx/v5/pgxpool"

type Store struct {
	Auth AuthStore
	Task TaskStore
}

func NewStore(pool *pgxpool.Pool) Store {
	return Store{
		Auth: newAuthStore(pool),
		Task: newTaskStore(pool),
	}
}
