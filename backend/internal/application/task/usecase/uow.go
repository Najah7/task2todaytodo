package usecase

import (
	"context"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

type Repositories interface {
	ProjectLifecycle() shared.ProjectWorkLifecycle
	TaskProjects() TaskProjectRepository
	Tasks() TaskRepository
	TaskTags() TaskTagRepository
	ActionItems() ActionItemRepository
	TodoLists() TodoListRepository
	TaskFrequencies() TaskFrequencyRepository
	TaskPriorities() TaskPriorityRepository
	TaskStatuses() TaskStatusRepository
}

type UOW interface {
	Do(ctx context.Context, fn func(context.Context, Repositories) error) error
}
