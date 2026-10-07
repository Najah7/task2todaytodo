package application

import (
	"context"

	scheduleusecase "github.com/Najah7/task2todaytodo/internal/application/schedule/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ScheduleUOW struct {
	pool  *pgxpool.Pool
	store ScheduleStore
}

func NewScheduleUOW(pool *pgxpool.Pool, store ScheduleStore) scheduleusecase.UOW {
	return &ScheduleUOW{pool: pool, store: store}
}

func (u *ScheduleUOW) Do(ctx context.Context, fn func(context.Context, scheduleusecase.Repositories) error) error {
	return RunInTx(ctx, u.pool, func(tx pgx.Tx) error {
		return fn(ctx, scheduleRepositories{store: u.store.WithTx(tx)})
	})
}

type scheduleRepositories struct{ store ScheduleStore }

func (r scheduleRepositories) Schedules() scheduleusecase.ScheduleRepository {
	return r.store.Schedules
}
