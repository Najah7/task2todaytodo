package usecase

import "context"

type Repositories interface {
	Projects() ProjectRepository
	ProjectTypes() ProjectTypeRepository
	Tasks() TaskRepository
	TaskTags() TaskTagRepository
	TodoItems() TodoItemRepository
	TodoLists() TodoListRepository
	TaskSchedules() TaskScheduleRepository
	TaskFrequencies() TaskFrequencyRepository
	TaskPriorities() TaskPriorityRepository
	TaskStatuses() TaskStatusRepository
}

type UOW interface {
	Do(ctx context.Context, fn func(context.Context, Repositories) error) error
}
