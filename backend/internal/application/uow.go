package application

import (
	"time"

	allocationrepository "github.com/Najah7/task2todaytodo/internal/application/allocation/repository"
	allocationusecase "github.com/Najah7/task2todaytodo/internal/application/allocation/usecase"
	authusecase "github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
	projectusecase "github.com/Najah7/task2todaytodo/internal/application/project/usecase"
	scheduleusecase "github.com/Najah7/task2todaytodo/internal/application/schedule/usecase"
	taskusecase "github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	todousecase "github.com/Najah7/task2todaytodo/internal/application/todo/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
)

type UOW struct {
	Auth           authusecase.UOW
	AuthAccount    authusecase.UOW
	Project        projectusecase.UnitOfWork
	ProjectRead    projectusecase.UnitOfWork
	Task           taskusecase.UOW
	TaskRead       taskusecase.UOW
	Schedule       scheduleusecase.UOW
	ScheduleRead   scheduleusecase.UOW
	Allocation     allocationusecase.UnitOfWork
	AllocationRead allocationusecase.UnitOfWork
	Todo           todousecase.UOW
}

func NewUOW(pool *pgxpool.Pool, authStore AuthStore, projectStore ProjectStore, taskStore TaskStore, scheduleStore ScheduleStore, allocationStore *allocationrepository.AllocationRepository, availability ...func(pgx.Tx) authusecase.UserAvailabilityValidator) UOW {
	return NewUOWWithID(pool, authStore, projectStore, taskStore, scheduleStore, allocationStore, nil, availability...)
}

func NewUOWWithID(pool *pgxpool.Pool, authStore AuthStore, projectStore ProjectStore, taskStore TaskStore, scheduleStore ScheduleStore, allocationStore *allocationrepository.AllocationRepository, newID func() string, availability ...func(pgx.Tx) authusecase.UserAvailabilityValidator) UOW {
	if newID == nil {
		newID = func() string { return ulid.Make().String() }
	}
	if len(availability) == 0 && allocationStore != nil {
		availability = append(availability, func(tx pgx.Tx) authusecase.UserAvailabilityValidator {
			return allocationusecase.NewProfileAvailabilityValidator(allocationStore.WithTx(tx), time.Now)
		})
	}
	store := Store{Auth: authStore, Project: projectStore, Task: taskStore, Schedule: scheduleStore, Allocation: allocationStore}
	return UOW{
		Auth:           NewAuthUOW(pool, authStore, availability...),
		AuthAccount:    NewAuthAccountUOW(pool, authStore),
		Project:        NewProjectUOW(pool, projectStore, taskStore, scheduleStore),
		ProjectRead:    NewProjectReadUOW(pool, projectStore, taskStore, scheduleStore),
		Task:           NewTaskUOW(pool, taskStore, projectStore, scheduleStore, newID),
		TaskRead:       NewTaskReadUOW(pool, taskStore, projectStore, scheduleStore, newID),
		Schedule:       NewScheduleUOW(pool, scheduleStore, projectStore, store, taskStore, newID),
		ScheduleRead:   NewScheduleReadUOW(pool, scheduleStore, projectStore, store, taskStore, newID),
		Allocation:     NewAllocationUOW(pool, allocationStore),
		AllocationRead: NewAllocationReadUOW(pool, allocationStore),
		Todo:           NewTodoUOW(pool, store, taskStore, projectStore, scheduleStore, newID, nil),
	}
}
