package usecase

import "context"

type UnitOfWork interface {
	Do(ctx context.Context, fn func(ctx context.Context, repos Repositories) error) error
}

type Repositories interface {
	Projects() ProjectRepository
	ProjectTypes() ProjectTypeRepository
	Tasks() TaskRepository
	TodoItems() TodoItemRepository
	TodoLists() TodoListRepository
	TaskSchedules() TaskScheduleRepository
	TaskFrequencies() TaskFrequencyRepository
	TaskPriorities() TaskPriorityRepository
	TaskStatuses() TaskStatusRepository
}
