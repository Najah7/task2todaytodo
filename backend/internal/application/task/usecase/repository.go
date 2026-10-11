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
	ErrTaskNotFound                = errors.New("task not found")
	ErrTaskProjectionUnavailable   = errors.New("Task ActionItem projection reader is unavailable")
	ErrTaskProjectNotFound         = errors.New("project not found")
	ErrProjectLifecycleUnavailable = errors.New("project lifecycle service is unavailable")
	ErrActionItemTaskNotFound      = errors.New("action item task not found")
	ErrActionItemNotFound          = errors.New("action item not found")
	ErrRevisionConflict            = errors.New("resource revision does not match")
	ErrPermissionDenied            = errors.New("permission denied")
)

// TaskProject is the small, primitive project view needed by Task commands.
// Project lifecycle, membership, and progress belong to the Project context.
type TaskProject struct {
	ID              string
	OwnerID         string
	Title           string
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
	BumpRevisionByUserID(context.Context, domain.UserID, domain.TaskID, int32, shared.Capability) error
	Get(ctx context.Context, id domain.TaskID) (dao.Task, error)
	GetByUserID(ctx context.Context, userID domain.UserID, id domain.TaskID) (dao.Task, error)
	GetByUserIDWithPermission(ctx context.Context, userID domain.UserID, id domain.TaskID, capability shared.Capability) (dao.Task, error)
	GetDetailsByUserID(ctx context.Context, userID domain.UserID, id domain.TaskID) (dao.TaskDetails, error)
	AssignToProjectByUserID(ctx context.Context, userID domain.UserID, taskID domain.TaskID, projectID domain.ProjectID, expectedRevision int32) (dao.Task, error)
	RemoveFromProjectByUserID(ctx context.Context, userID domain.UserID, taskID domain.TaskID, projectID domain.ProjectID, expectedRevision int32) (dao.Task, error)
	CreateInProject(ctx context.Context, actorID domain.UserID, task domain.Task) (dao.Task, error)
	ReassignProjectMemberTasks(ctx context.Context, actor domain.UserID, project domain.ProjectID, member domain.UserID) error
	DeleteProjectTasksByActor(ctx context.Context, actorID, projectID string) error
	UpdateTaskAssigneeByActor(ctx context.Context, actorID domain.UserID, taskID domain.TaskID, assigneeID domain.UserID, expectedRevision int32) (dao.Task, error)
	ListEligibleTaskAssignees(ctx context.Context, actorID domain.UserID, taskID domain.TaskID) ([]dao.TaskAssignee, error)
	LockProjectForTaskAssignment(ctx context.Context, actorID domain.UserID, taskID domain.TaskID) error
	IsEligibleTaskAssignee(ctx context.Context, actorID domain.UserID, taskID domain.TaskID, assigneeID domain.UserID) (bool, error)
	GetByPriority(ctx context.Context, priority domain.TaskPriority) ([]dao.Task, error)
	GetByProject(ctx context.Context, projectID domain.ProjectID) ([]dao.Task, error)
	GetByStatus(ctx context.Context, status domain.TaskStatus) ([]dao.Task, error)
	Create(ctx context.Context, task domain.Task) (dao.Task, error)
	Update(ctx context.Context, task domain.Task) (dao.Task, error)
	UpdateByUserID(ctx context.Context, userID domain.UserID, task domain.Task, expectedRevision int32) (dao.Task, error)
	Delete(ctx context.Context, id domain.TaskID) error
	DeleteByUserID(ctx context.Context, userID domain.UserID, id domain.TaskID, expectedRevision int32) error
}

type TaskPriorityRepository interface {
	List(ctx context.Context) ([]dao.Priority, error)
}

type TaskStatusRepository interface {
	List(ctx context.Context) ([]dao.TaskStatus, error)
}

type ActionItemRepository interface {
	TaskListProjectionReader
	GetForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.ActionItemID) (dao.ActionItem, error)
	GetForCommand(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.ActionItemID, capability shared.Capability) (dao.ActionItem, error)
	ListByTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID) ([]dao.ActionItem, error)
	ListByTaskCursor(ctx context.Context, userID domain.UserID, taskID domain.TaskID, limit int, anchor *CursorAnchor) ([]dao.ActionItem, error)
	CreateForOwnedTask(ctx context.Context, userID domain.UserID, item domain.ActionItem, appendToTail bool) (dao.ActionItem, error)
	UpdateForOwnedTask(ctx context.Context, userID domain.UserID, item domain.ActionItem) (dao.ActionItem, error)
	ReorderForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.ActionItemID, newPosition int) (dao.ActionItem, error)
	CheckForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.ActionItemID) error
	UncheckForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.ActionItemID) error
	DeleteForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.ActionItemID) error
}

type TodoListRepository interface {
	Get(ctx context.Context, id domain.TodoListID) (dao.TodoList, error)
	GetByUserIDAndDate(ctx context.Context, userID domain.UserID, listDate time.Time) (dao.TodoList, error)
	ListByUserID(ctx context.Context, userID domain.UserID) ([]dao.TodoList, error)
	Create(ctx context.Context, list domain.TodoList) (dao.TodoList, error)
	Delete(ctx context.Context, id domain.TodoListID) error
}
