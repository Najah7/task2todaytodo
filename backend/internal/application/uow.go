package application

import (
	authusecase "github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
	taskusecase "github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UOW struct {
	Auth authusecase.UOW
	Task taskusecase.UOW
}

func NewUOW(pool *pgxpool.Pool, authStore AuthStore, taskStore TaskStore) UOW {
	return UOW{
		Auth: NewAuthUOW(pool, authStore),
		Task: NewTaskUOW(pool, taskStore),
	}
}
