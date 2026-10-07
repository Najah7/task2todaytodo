package application

import (
	"github.com/Najah7/task2todaytodo/internal/application/schedule/repository"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ScheduleStore struct {
	Schedules *repository.ScheduleRepository
}

func newScheduleStore(pool *pgxpool.Pool) ScheduleStore {
	return ScheduleStore{Schedules: repository.NewScheduleRepository(pool)}
}

func (s ScheduleStore) WithTx(tx pgx.Tx) ScheduleStore {
	return ScheduleStore{Schedules: s.Schedules.WithTx(tx)}
}
