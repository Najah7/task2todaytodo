package application

import (
	"context"

	allocationrepository "github.com/Najah7/task2todaytodo/internal/application/allocation/repository"
	projectusecase "github.com/Najah7/task2todaytodo/internal/application/project/usecase"
	scheduleusecase "github.com/Najah7/task2todaytodo/internal/application/schedule/usecase"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ScheduleUOW struct {
	pool     *pgxpool.Pool
	store    ScheduleStore
	projects ProjectStore
}

func NewScheduleUOW(pool *pgxpool.Pool, store ScheduleStore, projects ProjectStore) scheduleusecase.UOW {
	return &ScheduleUOW{pool: pool, store: store, projects: projects}
}

func (u *ScheduleUOW) Do(ctx context.Context, fn func(context.Context, scheduleusecase.Repositories) error) error {
	return RunInTx(ctx, u.pool, func(tx pgx.Tx) error {
		projectRepository := u.projects.WithTx(tx).Projects
		scheduleStore := u.store.WithTx(tx)
		allocationRules := allocationrepository.NewAllocationRepository(u.pool).WithTx(tx)
		return fn(ctx, scheduleRepositories{
			store:     scheduleStore,
			lifecycle: projectusecase.NewProjectLifecycleUseCase(projectRepository, projectRepository, scheduleStore.Schedules, allocationProjectLifecycleAdapter{repository: allocationRules}),
		})
	})
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
