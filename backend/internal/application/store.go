package application

import (
	allocationrepository "github.com/Najah7/task2todaytodo/internal/application/allocation/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	Auth       AuthStore
	Project    ProjectStore
	Task       TaskStore
	Schedule   ScheduleStore
	Admin      AdminStore
	Inbox      InboxStore
	Allocation *allocationrepository.AllocationRepository
}

func NewStore(pool *pgxpool.Pool) Store {
	return Store{
		Auth:       newAuthStore(pool),
		Project:    newProjectStore(pool),
		Task:       newTaskStore(pool),
		Schedule:   newScheduleStore(pool),
		Admin:      newAdminStore(pool),
		Inbox:      newInboxStore(pool),
		Allocation: allocationrepository.NewAllocationRepository(pool),
	}
}
