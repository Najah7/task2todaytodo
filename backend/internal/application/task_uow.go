package application

import (
	"context"
	"errors"
	"fmt"

	allocationrepository "github.com/Najah7/task2todaytodo/internal/application/allocation/repository"
	projectusecase "github.com/Najah7/task2todaytodo/internal/application/project/usecase"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	todousecase "github.com/Najah7/task2todaytodo/internal/application/todo/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
)

type TaskUOW struct {
	pool      *pgxpool.Pool
	store     TaskStore
	projects  ProjectStore
	schedules ScheduleStore
	newID     func() string
	runTx     transactionRunner
}

func NewTaskUOW(pool *pgxpool.Pool, store TaskStore, projects ProjectStore, schedules ScheduleStore, ids ...func() string) usecase.UOW {
	return newTaskUOW(pool, store, projects, schedules, RunInTodoWorkflowTx, ids...)
}

func NewTaskReadUOW(pool *pgxpool.Pool, store TaskStore, projects ProjectStore, schedules ScheduleStore, ids ...func() string) usecase.UOW {
	return newTaskUOW(pool, store, projects, schedules, RunInTx, ids...)
}

func newTaskUOW(pool *pgxpool.Pool, store TaskStore, projects ProjectStore, schedules ScheduleStore, runTx transactionRunner, ids ...func() string) usecase.UOW {
	var newID func() string
	if len(ids) > 0 {
		newID = ids[0]
	}
	if newID == nil {
		newID = func() string { return ulid.Make().String() }
	}
	return &TaskUOW{pool: pool, store: store, projects: projects, schedules: schedules, newID: newID, runTx: runTx}
}

func (u *TaskUOW) Do(
	ctx context.Context,
	fn func(ctx context.Context, repos usecase.Repositories) error,
) error {
	return u.runTx(ctx, u.pool, func(tx pgx.Tx) error {
		projects := u.projects.WithTx(tx).Projects
		store := u.store.WithTx(tx)
		scheduleRules := u.schedules.WithTx(tx).Schedules
		allocationRules := allocationrepository.NewAllocationRepository(u.pool).WithTx(tx)
		todoRepos := newTodoRepositoriesFromTx(store, u.projects, u.schedules, tx, u.newID, nil)
		lifecycle := taskTodoLifecycle{items: todoRepos.Items(), todos: todoRepos.Todos(), sources: todoRepos.Sources(), repos: todoRepos,
			regenerator: todousecase.NewRegenerateTodayUseCase(nil, u.newID)}
		return fn(ctx, taskRepositories{
			store:     store,
			lifecycle: projectusecase.NewProjectLifecycleUseCase(projects, projects, scheduleRules, allocationProjectLifecycleAdapter{repository: allocationRules}),
			todos:     lifecycle,
		})
	})
}

type taskRepositories struct {
	store     TaskStore
	lifecycle shared.ProjectWorkLifecycle
	todos     taskTodoLifecycle
}

func (r taskRepositories) ProjectLifecycle() shared.ProjectWorkLifecycle { return r.lifecycle }

func (r taskRepositories) TaskProjects() usecase.TaskProjectRepository {
	return r.store.Tasks
}

func (r taskRepositories) Tasks() usecase.TaskRepository {
	return r.store.Tasks
}

func (r taskRepositories) ActionItems() usecase.ActionItemRepository {
	return r.store.ActionItems
}

func (r taskRepositories) TaskPriorities() usecase.TaskPriorityRepository {
	return r.store.TaskPriorities
}

func (r taskRepositories) TaskStatuses() usecase.TaskStatusRepository {
	return r.store.TaskStatuses
}

func (r taskRepositories) ActionItemTodos() usecase.ActionItemTodoLifecycle { return r.todos }
func (r taskRepositories) TaskTodos() usecase.TaskTodoLifecycle             { return r.todos }

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

type transactionRunner func(context.Context, *pgxpool.Pool, func(pgx.Tx) error) error

func RunInTodoWorkflowTx(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) error {
	return RunInTx(ctx, pool, func(tx pgx.Tx) error {
		if err := lockTodoWorkflow(ctx, tx); err != nil {
			return fmt.Errorf("acquire todo workflow lock: %w", err)
		}
		return fn(tx)
	})
}
