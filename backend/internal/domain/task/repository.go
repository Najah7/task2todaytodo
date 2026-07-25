package task

import (
	"context"
	"time"
)

type ProjectRepository interface {
	Get(ctx context.Context, id ProjectID) (Project, error)
	Create(ctx context.Context, project Project) (Project, error)
	Update(ctx context.Context, project Project) (Project, error)
	Delete(ctx context.Context, id ProjectID) error
}

type ProjectTypeRepository interface {
	List(ctx context.Context) ([]ProjectType, error)
}

type TaskRepository interface {
	Get(ctx context.Context, id TaskID) (Task, error)
	GetByFrequency(ctx context.Context, frequency TaskFrequency) ([]Task, error)
	GetByPriority(ctx context.Context, priority TaskPriority) ([]Task, error)
	GetByProject(ctx context.Context, projectID ProjectID) ([]Task, error)
	GetByProjectType(ctx context.Context, projectType ProjectType) ([]Task, error)
	GetByStatus(ctx context.Context, status TaskStatus) ([]Task, error)
	GetByTag(ctx context.Context, tagID string) ([]Task, error)
	Create(ctx context.Context, task Task) (Task, error)
	Update(ctx context.Context, task Task) (Task, error)
	Delete(ctx context.Context, id TaskID) error
}

type TaskFrequencyRepository interface {
	List(ctx context.Context) ([]TaskFrequency, error)
}

type TaskPriorityRepository interface {
	List(ctx context.Context) ([]TaskPriority, error)
}

type TaskStatusRepository interface {
	List(ctx context.Context) ([]TaskStatus, error)
}

type TaskScheduleRepository interface {
	Get(ctx context.Context, id TaskScheduleID) (TaskSchedule, error)
	Create(ctx context.Context, schedule TaskSchedule) (TaskSchedule, error)
	Update(ctx context.Context, schedule TaskSchedule) (TaskSchedule, error)
	Delete(ctx context.Context, id TaskScheduleID) error
}

type TodoItemRepository interface {
	Get(ctx context.Context, id TodoItemID) (TodoItem, error)
	Create(ctx context.Context, item TodoItem) (TodoItem, error)
	Update(ctx context.Context, item TodoItem) (TodoItem, error)
	Delete(ctx context.Context, id TodoItemID) error
}

type TodoListRepository interface {
	Get(ctx context.Context, id TodoListID) (TodoList, error)
	GetByUserAndDate(ctx context.Context, userID UserID, listDate time.Time) (TodoList, error)
	ListByUser(ctx context.Context, userID UserID) ([]TodoList, error)
	Create(ctx context.Context, list TodoList) (TodoList, error)
	Delete(ctx context.Context, id TodoListID) error
}
