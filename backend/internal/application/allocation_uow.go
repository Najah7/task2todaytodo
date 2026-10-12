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
	runTx transactionRunner
}

func NewAllocationUOW(pool *pgxpool.Pool, store *allocationrepository.AllocationRepository) allocationusecase.UnitOfWork {
	return newAllocationUOW(pool, store, RunInTodoWorkflowTx)
}

func NewAllocationReadUOW(pool *pgxpool.Pool, store *allocationrepository.AllocationRepository) allocationusecase.UnitOfWork {
	return newAllocationUOW(pool, store, RunInTx)
}

func newAllocationUOW(pool *pgxpool.Pool, store *allocationrepository.AllocationRepository, runTx transactionRunner) *AllocationUOW {
	return &AllocationUOW{pool: pool, store: store, runTx: runTx}
}

func (u *AllocationUOW) Do(ctx context.Context, fn func(context.Context, allocationusecase.Repository) error) error {
	return u.runTx(ctx, u.pool, func(tx pgx.Tx) error {
		return fn(ctx, u.store.WithTx(tx))
	})
}
