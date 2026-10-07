package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TaskUOW struct {
	pool  *pgxpool.Pool
	store TaskStore
}

func NewTaskUOW(pool *pgxpool.Pool, store TaskStore) usecase.UOW {
	return &TaskUOW{
		pool:  pool,
		store: store,
	}
}

func (u *TaskUOW) Do(
	ctx context.Context,
	fn func(ctx context.Context, repos usecase.Repositories) error,
) error {
	return RunInTx(ctx, u.pool, func(tx pgx.Tx) error {
		return fn(ctx, taskRepositories{store: u.store.WithTx(tx)})
	})
}

type taskRepositories struct {
	store TaskStore
}

func (r taskRepositories) TaskProjects() usecase.TaskProjectRepository {
	return r.store.Tasks
}

func (r taskRepositories) Tasks() usecase.TaskRepository {
	return r.store.Tasks
}

func (r taskRepositories) TaskTags() usecase.TaskTagRepository {
	return r.store.TaskTags
}

func (r taskRepositories) TodoItems() usecase.TodoItemRepository {
	return r.store.TodoItems
}

func (r taskRepositories) TodoLists() usecase.TodoListRepository {
	return r.store.TodoLists
}

func (r taskRepositories) TaskFrequencies() usecase.TaskFrequencyRepository {
	return r.store.TaskFrequencies
}

func (r taskRepositories) TaskPriorities() usecase.TaskPriorityRepository {
	return r.store.TaskPriorities
}

func (r taskRepositories) TaskStatuses() usecase.TaskStatusRepository {
	return r.store.TaskStatuses
}

func RunInTx(
	ctx context.Context,
	pool *pgxpool.Pool,
	fn func(tx pgx.Tx) error,
) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			_ = tx.Rollback(ctx)
			panic(recovered)
		}
	}()

	if err := fn(tx); err != nil {
		rollbackErr := tx.Rollback(ctx)
		if rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			return errors.Join(
				err,
				fmt.Errorf("rollback transaction: %w", rollbackErr),
			)
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
