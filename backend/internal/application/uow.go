package application

import (
	allocationrepository "github.com/Najah7/task2todaytodo/internal/application/allocation/repository"
	allocationusecase "github.com/Najah7/task2todaytodo/internal/application/allocation/usecase"
	authusecase "github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
	projectusecase "github.com/Najah7/task2todaytodo/internal/application/project/usecase"
	scheduleusecase "github.com/Najah7/task2todaytodo/internal/application/schedule/usecase"
	taskusecase "github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type UOW struct {
	Auth       authusecase.UOW
	Project    projectusecase.UnitOfWork
	Task       taskusecase.UOW
	Schedule   scheduleusecase.UOW
	Allocation allocationusecase.UnitOfWork
}

func NewUOW(pool *pgxpool.Pool, authStore AuthStore, projectStore ProjectStore, taskStore TaskStore, scheduleStore ScheduleStore, allocationStore *allocationrepository.AllocationRepository, availability ...func(pgx.Tx) authusecase.UserAvailabilityValidator) UOW {
	if len(availability) == 0 && allocationStore != nil {
		availability = append(availability, func(tx pgx.Tx) authusecase.UserAvailabilityValidator {
			return allocationusecase.NewProfileAvailabilityValidator(allocationStore.WithTx(tx), time.Now)
		})
	}
	return UOW{
		Auth:       NewAuthUOW(pool, authStore, availability...),
		Project:    NewProjectUOW(pool, projectStore, taskStore, scheduleStore),
		Task:       NewTaskUOW(pool, taskStore, projectStore, scheduleStore),
		Schedule:   NewScheduleUOW(pool, scheduleStore, projectStore),
		Allocation: NewAllocationUOW(pool, allocationStore),
	}
}
