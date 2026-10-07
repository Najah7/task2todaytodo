package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

var (
	ErrTaskNotFound              = errors.New("task not found")
	ErrTaskProjectNotFound       = errors.New("project not found")
	ErrTodoItemTaskNotFound      = errors.New("todo item task not found")
	ErrTodoItemNotFound          = errors.New("todo item not found")
	ErrTodoItemPositionConflict  = errors.New("todo item position already exists")
	ErrOccurrenceDateRequired    = errors.New("occurrence date is required for recurring series")
	ErrOccurrenceNotFound        = errors.New("recurrence occurrence not found")
	ErrOccurrenceInactive        = errors.New("recurrence occurrence is inactive")
	ErrOccurrenceCompleted       = errors.New("completed occurrence cannot be skipped")
	ErrOccurrenceRuleMismatch    = errors.New("date does not match active recurrence rule")
	ErrTaskTagNotFound           = errors.New("task tag not found")
	ErrTaskTagAssignmentNotOwned = errors.New("task and tag must belong to the same user")
	ErrRevisionConflict          = errors.New("resource revision does not match")
	ErrPermissionDenied          = errors.New("permission denied")
)

type UserTimezoneReader interface {
	GetTimezone(ctx context.Context, userID string) (string, error)
}

// TaskProject is the small, primitive project view needed by Task commands.
// Project lifecycle, membership, and progress belong to the Project context.
type TaskProject struct {
	ID              string
	OwnerID         string
	DefaultPriority string
}

type TaskProjectRepository interface {
	GetProjectByUserID(ctx context.Context, actorID, projectID string) (TaskProject, error)
	GetProjectByUserIDWithPermission(ctx context.Context, actorID, projectID string, capability shared.Capability) (TaskProject, error)
	LockProjectByUserIDWithPermission(ctx context.Context, actorID, projectID string, capability shared.Capability) (TaskProject, error)
}

type taskProgressSource interface {
	ReadTaskProgressSources(ctx context.Context, taskIDs []string, asOf time.Time) (dao.TaskProgressSources, error)
}

type taskStatusMutationRepository interface {
	LockByUserID(context.Context, domain.UserID, domain.TaskID) (dao.Task, error)
	SetStatusByUserID(context.Context, domain.UserID, domain.TaskID, domain.TaskStatus) error
}

type TaskRepository interface {
	taskProgressSource
	taskStatusMutationRepository
	LockByUserIDWithPermission(context.Context, domain.UserID, domain.TaskID, shared.Capability) (dao.Task, error)
	HasPermission(context.Context, domain.UserID, domain.TaskID, shared.Capability) (bool, error)
	SetStatusByUserIDWithPermission(context.Context, domain.UserID, domain.TaskID, domain.TaskStatus, int32, shared.Capability) error
	Get(ctx context.Context, id domain.TaskID) (dao.Task, error)
	GetByUserID(ctx context.Context, userID domain.UserID, id domain.TaskID) (dao.Task, error)
	GetByUserIDWithPermission(ctx context.Context, userID domain.UserID, id domain.TaskID, capability shared.Capability) (dao.Task, error)
	GetDetailsByUserID(ctx context.Context, userID domain.UserID, id domain.TaskID) (dao.TaskDetails, error)
	ListUsersWithActiveRecurrences(ctx context.Context) ([]domain.UserID, error)
	AssignToProjectByUserID(ctx context.Context, userID domain.UserID, taskID domain.TaskID, projectID domain.ProjectID, expectedRevision int32) (dao.Task, error)
	RemoveFromProjectByUserID(ctx context.Context, userID domain.UserID, taskID domain.TaskID, projectID domain.ProjectID, expectedRevision int32) (dao.Task, error)
	CreateInProject(ctx context.Context, actorID domain.UserID, task domain.Task) (dao.Task, error)
	ReassignProjectMemberTasks(ctx context.Context, actor domain.UserID, project domain.ProjectID, member domain.UserID) error
	DeleteProjectTasksByActor(ctx context.Context, actorID, projectID string) error
	UpdateTaskAssigneeByActor(ctx context.Context, actorID domain.UserID, taskID domain.TaskID, assigneeID domain.UserID, expectedRevision int32) (dao.Task, error)
	ListEligibleTaskAssignees(ctx context.Context, actorID domain.UserID, taskID domain.TaskID) ([]dao.TaskAssignee, error)
	LockProjectForTaskAssignment(ctx context.Context, actorID domain.UserID, taskID domain.TaskID) error
	IsEligibleTaskAssignee(ctx context.Context, actorID domain.UserID, taskID domain.TaskID, assigneeID domain.UserID) (bool, error)
	GetByFrequency(ctx context.Context, frequency domain.TaskFrequency) ([]dao.Task, error)
	GetByPriority(ctx context.Context, priority domain.TaskPriority) ([]dao.Task, error)
	GetByProject(ctx context.Context, projectID domain.ProjectID) ([]dao.Task, error)
	GetByStatus(ctx context.Context, status domain.TaskStatus) ([]dao.Task, error)
	GetByTag(ctx context.Context, tagID string) ([]dao.Task, error)
	Create(ctx context.Context, task domain.Task) (dao.Task, error)
	Update(ctx context.Context, task domain.Task) (dao.Task, error)
	UpdateByUserID(ctx context.Context, userID domain.UserID, task domain.Task, expectedRevision int32) (dao.Task, error)
	Delete(ctx context.Context, id domain.TaskID) error
	DeleteByUserID(ctx context.Context, userID domain.UserID, id domain.TaskID, expectedRevision int32) error
}

type TaskTagRepository interface {
	ListByTaskAndUserID(ctx context.Context, userID domain.UserID, taskID domain.TaskID) ([]dao.TaskTag, error)
	AddToTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, tagID string) error
	RemoveFromTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, tagID string) error
}

type TaskFrequencyRepository interface {
	List(ctx context.Context) ([]dao.TaskFrequency, error)
}

type TaskPriorityRepository interface {
	List(ctx context.Context) ([]dao.Priority, error)
}

type TaskStatusRepository interface {
	List(ctx context.Context) ([]dao.TaskStatus, error)
}

type TodoItemRepository interface {
	Get(ctx context.Context, id domain.TodoItemID) (dao.TodoItem, error)
	GetForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TodoItemID) (dao.TodoItem, error)
	ListByTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID) ([]dao.TodoItem, error)
	ListActiveSeriesByUserID(ctx context.Context, userID domain.UserID) ([]dao.TodoItem, error)
	Create(ctx context.Context, item domain.TodoItem) (dao.TodoItem, error)
	CreateForOwnedTask(ctx context.Context, userID domain.UserID, item domain.TodoItem, appendToTail bool) (dao.TodoItem, error)
	CreateOccurrenceForOwnedTask(ctx context.Context, userID domain.UserID, item domain.TodoItem) (dao.TodoItem, error)
	Update(ctx context.Context, item domain.TodoItem) (dao.TodoItem, error)
	UpdateForOwnedTask(ctx context.Context, userID domain.UserID, item domain.TodoItem) (dao.TodoItem, error)
	SetPositionForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TodoItemID, position int) error
	ReorderForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TodoItemID, newPosition int) (dao.TodoItem, error)
	CheckForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TodoItemID) error
	UncheckForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TodoItemID) error
	Delete(ctx context.Context, id domain.TodoItemID) error
	DeleteForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TodoItemID) error
	TombstoneForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TodoItemID) error
	DeleteUneditedFutureBySeries(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.TodoItemID, fromDate time.Time) (int64, error)
	DeleteUneditedFutureByTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, now time.Time) (int64, error)
}

type TodoListRepository interface {
	Get(ctx context.Context, id domain.TodoListID) (dao.TodoList, error)
	GetByUserIDAndDate(ctx context.Context, userID domain.UserID, listDate time.Time) (dao.TodoList, error)
	ListByUserID(ctx context.Context, userID domain.UserID) ([]dao.TodoList, error)
	Create(ctx context.Context, list domain.TodoList) (dao.TodoList, error)
	Delete(ctx context.Context, id domain.TodoListID) error
}
