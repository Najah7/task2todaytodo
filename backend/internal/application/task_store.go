package application

import (
	"github.com/Najah7/task2todaytodo/internal/application/task/repository"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TaskStore struct {
	Projects        *repository.ProjectRepository
	Tasks           *repository.TaskRepository
	TaskTags        *repository.TaskTagRepository
	TodoItems       *repository.TodoItemRepository
	TodoLists       *repository.TodoListRepository
	TaskSchedules   *repository.TaskScheduleRepository
	ProjectTypes    *repository.ProjectTypeRepository
	TaskFrequencies *repository.TaskFrequencyRepository
	TaskPriorities  *repository.TaskPriorityRepository
	TaskStatuses    *repository.TaskStatusRepository
}

func newTaskStore(pool *pgxpool.Pool) TaskStore {
	return TaskStore{
		Projects:        repository.NewProjectRepository(pool),
		Tasks:           repository.NewTaskRepository(pool),
		TaskTags:        repository.NewTaskTagRepository(pool),
		TodoItems:       repository.NewTodoItemRepository(pool),
		TodoLists:       repository.NewTodoListRepository(pool),
		TaskSchedules:   repository.NewTaskScheduleRepository(pool),
		ProjectTypes:    repository.NewProjectTypeRepository(pool),
		TaskFrequencies: repository.NewTaskFrequencyRepository(pool),
		TaskPriorities:  repository.NewTaskPriorityRepository(pool),
		TaskStatuses:    repository.NewTaskStatusRepository(pool),
	}
}

func (s TaskStore) WithTx(tx pgx.Tx) TaskStore {
	return TaskStore{
		Projects:        s.Projects.WithTx(tx),
		Tasks:           s.Tasks.WithTx(tx),
		TaskTags:        s.TaskTags.WithTx(tx),
		TodoItems:       s.TodoItems.WithTx(tx),
		TodoLists:       s.TodoLists.WithTx(tx),
		TaskSchedules:   s.TaskSchedules.WithTx(tx),
		ProjectTypes:    s.ProjectTypes.WithTx(tx),
		TaskFrequencies: s.TaskFrequencies.WithTx(tx),
		TaskPriorities:  s.TaskPriorities.WithTx(tx),
		TaskStatuses:    s.TaskStatuses.WithTx(tx),
	}
}
