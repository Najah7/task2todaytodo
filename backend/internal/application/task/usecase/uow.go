package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

type Repositories interface {
	ProjectLifecycle() shared.ProjectWorkLifecycle
	TaskProjects() TaskProjectRepository
	Tasks() TaskRepository
	ActionItems() ActionItemRepository
	ActionItemTodos() ActionItemTodoLifecycle
	TaskTodos() TaskTodoLifecycle
	TaskPriorities() TaskPriorityRepository
	TaskStatuses() TaskStatusRepository
}

type UOW interface {
	Do(ctx context.Context, fn func(context.Context, Repositories) error) error
}
