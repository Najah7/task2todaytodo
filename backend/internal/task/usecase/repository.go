package usecase

import (
	"context"
	"errors"
	"time"

	domain "github.com/Najah7/task2todaytodo/internal/task/domain"
)

var (
	ErrTaskNotFound             = errors.New("task not found")
	ErrTaskProjectNotFound      = errors.New("project not found")
	ErrTodoItemTaskNotFound     = errors.New("todo item task not found")
	ErrTodoItemNotFound         = errors.New("todo item not found")
	ErrTodoItemPositionConflict = errors.New("todo item position already exists")
)

type ProjectRepository interface {
	GetByUser(ctx context.Context, userID domain.UserID, id domain.ProjectID) (domain.Project, error)
	GetAggregateByUser(ctx context.Context, userID domain.UserID, id domain.ProjectID) (domain.ProjectAggregate, error)
	Create(ctx context.Context, project domain.Project) (domain.Project, error)
	UpdateByUser(ctx context.Context, userID domain.UserID, project domain.Project) (domain.Project, error)
	DeleteByUser(ctx context.Context, userID domain.UserID, id domain.ProjectID) error
}

type ProjectTypeRepository interface {
	List(ctx context.Context) ([]domain.ProjectType, error)
}

type TaskRepository interface {
	Get(ctx context.Context, id domain.TaskID) (domain.Task, error)
	GetByUser(ctx context.Context, userID domain.UserID, id domain.TaskID) (domain.Task, error)
	GetAggregateByUser(ctx context.Context, userID domain.UserID, id domain.TaskID) (domain.TaskAggregate, error)
	GetByFrequency(ctx context.Context, frequency domain.TaskFrequency) ([]domain.Task, error)
	GetByPriority(ctx context.Context, priority domain.TaskPriority) ([]domain.Task, error)
	GetByProject(ctx context.Context, projectID domain.ProjectID) ([]domain.Task, error)
	GetByProjectType(ctx context.Context, projectType domain.ProjectType) ([]domain.Task, error)
	GetByStatus(ctx context.Context, status domain.TaskStatus) ([]domain.Task, error)
	GetByTag(ctx context.Context, tagID string) ([]domain.Task, error)
	Create(ctx context.Context, task domain.Task) (domain.Task, error)
	CreateInProject(ctx context.Context, task domain.Task) (domain.Task, error)
	Update(ctx context.Context, task domain.Task) (domain.Task, error)
	UpdateByUser(ctx context.Context, task domain.Task) (domain.Task, error)
	Delete(ctx context.Context, id domain.TaskID) error
	DeleteByUser(ctx context.Context, userID domain.UserID, id domain.TaskID) error
}

type TaskFrequencyRepository interface {
	List(ctx context.Context) ([]domain.TaskFrequency, error)
}

type TaskPriorityRepository interface {
	List(ctx context.Context) ([]domain.TaskPriority, error)
}

type TaskStatusRepository interface {
	List(ctx context.Context) ([]domain.TaskStatus, error)
}

type TaskScheduleRepository interface {
	Get(ctx context.Context, id domain.TaskScheduleID) (domain.TaskSchedule, error)
	GetByTaskAndUser(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TaskScheduleID) (domain.TaskSchedule, error)
	Create(ctx context.Context, schedule domain.TaskSchedule) (domain.TaskSchedule, error)
	CreateByTaskAndUser(ctx context.Context, userID domain.UserID, schedule domain.TaskSchedule) (domain.TaskSchedule, error)
	Update(ctx context.Context, schedule domain.TaskSchedule) (domain.TaskSchedule, error)
	UpdateByTaskAndUser(ctx context.Context, userID domain.UserID, schedule domain.TaskSchedule) (domain.TaskSchedule, error)
	Delete(ctx context.Context, id domain.TaskScheduleID) error
	DeleteByTaskAndUser(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TaskScheduleID) error
}

type TodoItemRepository interface {
	Get(ctx context.Context, id domain.TodoItemID) (domain.TodoItem, error)
	Create(ctx context.Context, item domain.TodoItem) (domain.TodoItem, error)
	CreateForOwnedTask(ctx context.Context, userID domain.UserID, item domain.TodoItem, appendToTail bool) (domain.TodoItem, error)
	Update(ctx context.Context, item domain.TodoItem) (domain.TodoItem, error)
	CheckForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TodoItemID) error
	UncheckForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TodoItemID) error
	Delete(ctx context.Context, id domain.TodoItemID) error
	DeleteForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TodoItemID) error
}

type TodoListRepository interface {
	Get(ctx context.Context, id domain.TodoListID) (domain.TodoList, error)
	GetByUserAndDate(ctx context.Context, userID domain.UserID, listDate time.Time) (domain.TodoList, error)
	ListByUser(ctx context.Context, userID domain.UserID) ([]domain.TodoList, error)
	Create(ctx context.Context, list domain.TodoList) (domain.TodoList, error)
	Delete(ctx context.Context, id domain.TodoListID) error
}
