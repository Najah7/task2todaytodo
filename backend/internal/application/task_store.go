package application

import (
	"github.com/Najah7/task2todaytodo/internal/application/task/repository"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TaskStore struct {
	Tasks          *repository.TaskRepository
	ActionItems    *repository.ActionItemRepository
	TaskPriorities *repository.TaskPriorityRepository
	TaskStatuses   *repository.TaskStatusRepository
}

func newTaskStore(pool *pgxpool.Pool) TaskStore {
	return TaskStore{
		Tasks:          repository.NewTaskRepository(pool),
		ActionItems:    repository.NewActionItemRepository(pool),
		TaskPriorities: repository.NewTaskPriorityRepository(pool),
		TaskStatuses:   repository.NewTaskStatusRepository(pool),
	}
}

func (s TaskStore) WithTx(tx pgx.Tx) TaskStore {
	return TaskStore{
		Tasks:          s.Tasks.WithTx(tx),
		ActionItems:    s.ActionItems.WithTx(tx),
		TaskPriorities: s.TaskPriorities.WithTx(tx),
		TaskStatuses:   s.TaskStatuses.WithTx(tx),
	}
}
