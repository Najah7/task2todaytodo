package application

import (
	"github.com/Najah7/task2todaytodo/internal/application/task/repository"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TaskStore struct {
	Tasks           *repository.TaskRepository
	TaskTags        *repository.TaskTagRepository
	TodoItems       *repository.TodoItemRepository
	TodoLists       *repository.TodoListRepository
	TaskFrequencies *repository.TaskFrequencyRepository
	TaskPriorities  *repository.TaskPriorityRepository
	TaskStatuses    *repository.TaskStatusRepository
}

func newTaskStore(pool *pgxpool.Pool) TaskStore {
	return TaskStore{
		Tasks:           repository.NewTaskRepository(pool),
		TaskTags:        repository.NewTaskTagRepository(pool),
		TodoItems:       repository.NewTodoItemRepository(pool),
		TodoLists:       repository.NewTodoListRepository(pool),
		TaskFrequencies: repository.NewTaskFrequencyRepository(pool),
		TaskPriorities:  repository.NewTaskPriorityRepository(pool),
		TaskStatuses:    repository.NewTaskStatusRepository(pool),
	}
}

func (s TaskStore) WithTx(tx pgx.Tx) TaskStore {
	return TaskStore{
		Tasks:           s.Tasks.WithTx(tx),
		TaskTags:        s.TaskTags.WithTx(tx),
		TodoItems:       s.TodoItems.WithTx(tx),
		TodoLists:       s.TodoLists.WithTx(tx),
		TaskFrequencies: s.TaskFrequencies.WithTx(tx),
		TaskPriorities:  s.TaskPriorities.WithTx(tx),
		TaskStatuses:    s.TaskStatuses.WithTx(tx),
	}
}
