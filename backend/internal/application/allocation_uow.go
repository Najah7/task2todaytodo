package application

import (
	"context"

	allocationrepository "github.com/Najah7/task2todaytodo/internal/application/allocation/repository"
	allocationusecase "github.com/Najah7/task2todaytodo/internal/application/allocation/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AllocationUOW struct {
	pool  *pgxpool.Pool
	store *allocationrepository.AllocationRepository
}

func NewAllocationUOW(pool *pgxpool.Pool, store *allocationrepository.AllocationRepository) allocationusecase.UnitOfWork {
	return &AllocationUOW{pool: pool, store: store}
}

func (u *AllocationUOW) Do(ctx context.Context, fn func(context.Context, allocationusecase.Repository) error) error {
	return RunInTx(ctx, u.pool, func(tx pgx.Tx) error {
		return fn(ctx, u.store.WithTx(tx))
	})
}
