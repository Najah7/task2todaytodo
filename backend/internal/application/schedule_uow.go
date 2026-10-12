package application

import (
	"context"

	allocationrepository "github.com/Najah7/task2todaytodo/internal/application/allocation/repository"
	projectusecase "github.com/Najah7/task2todaytodo/internal/application/project/usecase"
	scheduleusecase "github.com/Najah7/task2todaytodo/internal/application/schedule/usecase"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
)

type ScheduleUOW struct {
	pool        *pgxpool.Pool
	store       ScheduleStore
	projects    ProjectStore
	sourceStore Store
	taskStore   TaskStore
	newID       func() string
	runTx       transactionRunner
}

func NewScheduleUOW(pool *pgxpool.Pool, store ScheduleStore, projects ProjectStore, sourceStore Store, taskStore TaskStore, newID func() string) scheduleusecase.UOW {
	return newScheduleUOW(pool, store, projects, sourceStore, taskStore, newID, RunInTodoWorkflowTx)
}

func NewScheduleReadUOW(pool *pgxpool.Pool, store ScheduleStore, projects ProjectStore, sourceStore Store, taskStore TaskStore, newID func() string) scheduleusecase.UOW {
	return newScheduleUOW(pool, store, projects, sourceStore, taskStore, newID, RunInTx)
}

func newScheduleUOW(pool *pgxpool.Pool, store ScheduleStore, projects ProjectStore, sourceStore Store, taskStore TaskStore, newID func() string, runTx transactionRunner) scheduleusecase.UOW {
	if newID == nil {
		newID = func() string { return ulid.Make().String() }
	}
	return &ScheduleUOW{pool: pool, store: store, projects: projects, sourceStore: sourceStore, taskStore: taskStore, newID: newID, runTx: runTx}
}

func (u *ScheduleUOW) Do(ctx context.Context, fn func(context.Context, scheduleusecase.Repositories) error) error {
	return u.runTx(ctx, u.pool, func(tx pgx.Tx) error {
		return fn(ctx, u.repositories(tx))
	})
}

func (u *ScheduleUOW) repositories(tx pgx.Tx) scheduleusecase.Repositories {
	projectRepository := u.projects.WithTx(tx).Projects
	scheduleStore := u.store.WithTx(tx)
	allocationRules := allocationrepository.NewAllocationRepository(u.pool).WithTx(tx)
	return scheduleRepositories{
		store: scheduleStore,
		lifecycle: projectusecase.NewProjectLifecycleUseCase(projectRepository, projectRepository, scheduleStore.Schedules,
			allocationProjectLifecycleAdapter{repository: allocationRules}),
	}
}

type scheduleRepositories struct {
	store     ScheduleStore
	lifecycle shared.ProjectWorkLifecycle
}

func (r scheduleRepositories) Schedules() scheduleusecase.ScheduleRepository {
	return r.store.Schedules
}

func (r scheduleRepositories) Rules() scheduleusecase.RuleRepository {
	return r.store.Schedules
}

func (r scheduleRepositories) ProjectLifecycle() shared.ProjectWorkLifecycle {
	return r.lifecycle
}
