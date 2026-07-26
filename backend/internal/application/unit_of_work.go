package application

import (
	"context"
	"errors"
	"fmt"

	authusecase "github.com/Najah7/task2todaytodo/internal/auth/usecase"
	taskusecase "github.com/Najah7/task2todaytodo/internal/task/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type authUnitOfWork struct {
	pool  *pgxpool.Pool
	store authStore
}

func newAuthUnitOfWork(pool *pgxpool.Pool, store authStore) authusecase.UnitOfWork {
	return &authUnitOfWork{
		pool:  pool,
		store: store,
	}
}

func (u *authUnitOfWork) Do(
	ctx context.Context,
	fn func(ctx context.Context, repos authusecase.Repositories) error,
) error {
	return RunInTx(ctx, u.pool, func(tx pgx.Tx) error {
		return fn(ctx, authRepositories{store: u.store.WithTx(tx)})
	})
}

type authRepositories struct {
	store authStore
}

func (r authRepositories) Users() authusecase.UserRepository {
	return r.store.Users
}

func (r authRepositories) AccessTokens() authusecase.AccessTokenRepository {
	return r.store.AccessTokens
}

type taskUnitOfWork struct {
	pool  *pgxpool.Pool
	store taskStore
}

func newTaskUnitOfWork(pool *pgxpool.Pool, store taskStore) taskusecase.UnitOfWork {
	return &taskUnitOfWork{
		pool:  pool,
		store: store,
	}
}

func (u *taskUnitOfWork) Do(
	ctx context.Context,
	fn func(ctx context.Context, repos taskusecase.Repositories) error,
) error {
	return RunInTx(ctx, u.pool, func(tx pgx.Tx) error {
		return fn(ctx, taskRepositories{store: u.store.WithTx(tx)})
	})
}

type taskRepositories struct {
	store taskStore
}

func (r taskRepositories) Projects() taskusecase.ProjectRepository {
	return r.store.Projects
}

func (r taskRepositories) ProjectTypes() taskusecase.ProjectTypeRepository {
	return r.store.ProjectTypes
}

func (r taskRepositories) Tasks() taskusecase.TaskRepository {
	return r.store.Tasks
}

func (r taskRepositories) TodoItems() taskusecase.TodoItemRepository {
	return r.store.TodoItems
}

func (r taskRepositories) TodoLists() taskusecase.TodoListRepository {
	return r.store.TodoLists
}

func (r taskRepositories) TaskSchedules() taskusecase.TaskScheduleRepository {
	return r.store.TaskSchedules
}

func (r taskRepositories) TaskFrequencies() taskusecase.TaskFrequencyRepository {
	return r.store.TaskFrequencies
}

func (r taskRepositories) TaskPriorities() taskusecase.TaskPriorityRepository {
	return r.store.TaskPriorities
}

func (r taskRepositories) TaskStatuses() taskusecase.TaskStatusRepository {
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
