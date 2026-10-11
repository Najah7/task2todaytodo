//go:build integration

package task_test

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/task/repository"
	"github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
)

type actionItemTestID struct{}

func (actionItemTestID) Generate() string { return ulid.Make().String() }

type actionItemTestRepositories struct {
	usecase.Repositories
	task       usecase.TaskRepository
	actionItem usecase.ActionItemRepository
}

func (r actionItemTestRepositories) Tasks() usecase.TaskRepository { return r.task }
func (r actionItemTestRepositories) ActionItems() usecase.ActionItemRepository {
	return r.actionItem
}

type actionItemTestUOW struct{ pool *pgxpool.Pool }

func (u actionItemTestUOW) Do(ctx context.Context, fn func(context.Context, usecase.Repositories) error) error {
	tx, err := u.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	repos := actionItemTestRepositories{
		task: repository.NewTaskRepository(tx), actionItem: repository.NewActionItemRepository(tx),
	}
	if err := fn(ctx, repos); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
