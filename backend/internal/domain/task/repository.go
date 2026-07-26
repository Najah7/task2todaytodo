package task

import (
	"context"
	"time"
)

type ProjectRepository interface {
	GetByUser(ctx context.Context, userID UserID, id ProjectID) (Project, error)
	GetAggregateByUser(ctx context.Context, userID UserID, id ProjectID) (ProjectAggregate, error)
	Create(ctx context.Context, project Project) (Project, error)
	UpdateByUser(ctx context.Context, userID UserID, project Project) (Project, error)
	DeleteByUser(ctx context.Context, userID UserID, id ProjectID) error
}

type ProjectTypeRepository interface {
	List(ctx context.Context) ([]ProjectType, error)
}

type TaskRepository interface {
	Get(ctx context.Context, id TaskID) (Task, error)
	GetByUser(ctx context.Context, userID UserID, id TaskID) (Task, error)
	GetAggregateByUser(ctx context.Context, userID UserID, id TaskID) (TaskAggregate, error)
	GetByFrequency(ctx context.Context, frequency TaskFrequency) ([]Task, error)
	GetByPriority(ctx context.Context, priority TaskPriority) ([]Task, error)
	GetByProject(ctx context.Context, projectID ProjectID) ([]Task, error)
	GetByProjectType(ctx context.Context, projectType ProjectType) ([]Task, error)
	GetByStatus(ctx context.Context, status TaskStatus) ([]Task, error)
	GetByTag(ctx context.Context, tagID string) ([]Task, error)
	Create(ctx context.Context, task Task) (Task, error)
	CreateInProject(ctx context.Context, task Task) (Task, error)
	Update(ctx context.Context, task Task) (Task, error)
	UpdateByUser(ctx context.Context, task Task) (Task, error)
	Delete(ctx context.Context, id TaskID) error
	DeleteByUser(ctx context.Context, userID UserID, id TaskID) error
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
	GetByTaskAndUser(ctx context.Context, userID UserID, taskID TaskID, id TaskScheduleID) (TaskSchedule, error)
	Create(ctx context.Context, schedule TaskSchedule) (TaskSchedule, error)
	CreateByTaskAndUser(ctx context.Context, userID UserID, schedule TaskSchedule) (TaskSchedule, error)
	Update(ctx context.Context, schedule TaskSchedule) (TaskSchedule, error)
	UpdateByTaskAndUser(ctx context.Context, userID UserID, schedule TaskSchedule) (TaskSchedule, error)
	Delete(ctx context.Context, id TaskScheduleID) error
	DeleteByTaskAndUser(ctx context.Context, userID UserID, taskID TaskID, id TaskScheduleID) error
}

type TodoItemRepository interface {
	Get(ctx context.Context, id TodoItemID) (TodoItem, error)
	Create(ctx context.Context, item TodoItem) (TodoItem, error)
	CreateForOwnedTask(ctx context.Context, userID UserID, item TodoItem, appendToTail bool) (TodoItem, error)
	Update(ctx context.Context, item TodoItem) (TodoItem, error)
	CheckForOwnedTask(ctx context.Context, userID UserID, taskID TaskID, id TodoItemID) error
	UncheckForOwnedTask(ctx context.Context, userID UserID, taskID TaskID, id TodoItemID) error
	Delete(ctx context.Context, id TodoItemID) error
	DeleteForOwnedTask(ctx context.Context, userID UserID, taskID TaskID, id TodoItemID) error
}

type TodoListRepository interface {
	Get(ctx context.Context, id TodoListID) (TodoList, error)
	GetByUserAndDate(ctx context.Context, userID UserID, listDate time.Time) (TodoList, error)
	ListByUser(ctx context.Context, userID UserID) ([]TodoList, error)
	Create(ctx context.Context, list TodoList) (TodoList, error)
	Delete(ctx context.Context, id TodoListID) error
}
