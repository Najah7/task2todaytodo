package usecase

import "context"

type Repositories interface {
	TaskProjects() TaskProjectRepository
	Tasks() TaskRepository
	TaskTags() TaskTagRepository
	TodoItems() TodoItemRepository
	TodoLists() TodoListRepository
	TaskFrequencies() TaskFrequencyRepository
	TaskPriorities() TaskPriorityRepository
	TaskStatuses() TaskStatusRepository
}

type UOW interface {
	Do(ctx context.Context, fn func(context.Context, Repositories) error) error
}
