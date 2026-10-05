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
	ErrTaskTagNameConflict       = errors.New("task tag name already exists for user")
	ErrTaskTagAssignmentNotOwned = errors.New("task and tag must belong to the same user")
	ErrRevisionConflict          = errors.New("resource revision does not match")
	ErrPermissionDenied          = errors.New("permission denied")
)

type UserTimezoneReader interface {
	GetTimezone(ctx context.Context, userID domain.UserID) (string, error)
}

type ProjectRepository interface {
	GetByUserID(ctx context.Context, userID domain.UserID, id domain.ProjectID) (dao.Project, error)
	GetByUserIDWithPermission(ctx context.Context, userID domain.UserID, id domain.ProjectID, capability shared.Capability) (dao.Project, error)
	ListByUserID(ctx context.Context, userID domain.UserID) ([]dao.Project, error)
	GetDetailsByUserID(ctx context.Context, userID domain.UserID, id domain.ProjectID) (dao.ProjectDetails, error)
	Create(ctx context.Context, project domain.Project) (dao.Project, error)
	UpdateByUserID(ctx context.Context, userID domain.UserID, project domain.Project, expectedRevision int32) (dao.Project, error)
	DeleteByUserID(ctx context.Context, userID domain.UserID, id domain.ProjectID, expectedRevision int32) error
	LockByUserIDWithPermission(ctx context.Context, userID domain.UserID, id domain.ProjectID, capability shared.Capability) (dao.Project, error)
	HasPermission(ctx context.Context, userID domain.UserID, id domain.ProjectID, capability shared.Capability) (bool, error)
	LockActiveTasksForDeletion(ctx context.Context, projectID domain.ProjectID) error
	LockProjectForMemberChange(ctx context.Context, project domain.ProjectID) error
	CheckProjectMemberUpsertPermission(ctx context.Context, actor domain.UserID, project domain.ProjectID, member domain.UserID) (bool, error)
	UpsertProjectMember(ctx context.Context, member domain.ProjectMember) error
	DeleteProjectMember(ctx context.Context, actor domain.UserID, project domain.ProjectID, member domain.UserID) error
	ListProjectMembers(ctx context.Context, actor domain.UserID, project domain.ProjectID) ([]dao.ProjectMember, error)
}

type taskProgressSource interface {
	ReadTaskProgressSources(ctx context.Context, taskIDs, projectIDs []string, asOf time.Time) (dao.TaskProgressSources, error)
}

type taskStatusMutationRepository interface {
	LockByUserID(context.Context, domain.UserID, domain.TaskID) (dao.Task, error)
	SetStatusByUserID(context.Context, domain.UserID, domain.TaskID, domain.TaskStatus) error
}

type ProjectTypeRepository interface {
	List(ctx context.Context) ([]dao.ProjectType, error)
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
	UpdateTaskAssigneeByActor(ctx context.Context, actorID domain.UserID, taskID domain.TaskID, assigneeID domain.UserID, expectedRevision int32) (dao.Task, error)
	ListEligibleTaskAssignees(ctx context.Context, actorID domain.UserID, taskID domain.TaskID) ([]dao.TaskAssignee, error)
	LockProjectForTaskAssignment(ctx context.Context, actorID domain.UserID, taskID domain.TaskID) error
	IsEligibleTaskAssignee(ctx context.Context, actorID domain.UserID, taskID domain.TaskID, assigneeID domain.UserID) (bool, error)
	GetByFrequency(ctx context.Context, frequency domain.TaskFrequency) ([]dao.Task, error)
	GetByPriority(ctx context.Context, priority domain.TaskPriority) ([]dao.Task, error)
	GetByProject(ctx context.Context, projectID domain.ProjectID) ([]dao.Task, error)
	GetByProjectType(ctx context.Context, projectType domain.ProjectType) ([]dao.Task, error)
	GetByStatus(ctx context.Context, status domain.TaskStatus) ([]dao.Task, error)
	GetByTag(ctx context.Context, tagID string) ([]dao.Task, error)
	Create(ctx context.Context, task domain.Task) (dao.Task, error)
	Update(ctx context.Context, task domain.Task) (dao.Task, error)
	UpdateByUserID(ctx context.Context, userID domain.UserID, task domain.Task, expectedRevision int32) (dao.Task, error)
	Delete(ctx context.Context, id domain.TaskID) error
	DeleteByUserID(ctx context.Context, userID domain.UserID, id domain.TaskID, expectedRevision int32) error
}

type TaskTagRepository interface {
	GetByUserID(ctx context.Context, userID domain.UserID, id domain.TaskTagID) (dao.TaskTag, error)
	ListByUserID(ctx context.Context, userID domain.UserID) ([]dao.TaskTag, error)
	ListByTaskAndUserID(ctx context.Context, userID domain.UserID, taskID domain.TaskID) ([]dao.TaskTag, error)
	Create(ctx context.Context, tag domain.TaskTag) (dao.TaskTag, error)
	RenameByUserID(ctx context.Context, userID domain.UserID, tag domain.TaskTag) (dao.TaskTag, error)
	DeleteByUserID(ctx context.Context, userID domain.UserID, id domain.TaskTagID) error
	AddToTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, tagID domain.TaskTagID) error
	RemoveFromTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, tagID domain.TaskTagID) error
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

type TaskScheduleRepository interface {
	Get(ctx context.Context, id domain.TaskScheduleID) (dao.TaskSchedule, error)
	GetByTaskAndUserID(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TaskScheduleID) (dao.TaskSchedule, error)
	ListByTaskAndUserID(ctx context.Context, userID domain.UserID, taskID domain.TaskID) ([]dao.TaskSchedule, error)
	ListActiveSeriesByUserID(ctx context.Context, userID domain.UserID) ([]dao.TaskSchedule, error)
	Create(ctx context.Context, schedule domain.TaskSchedule) (dao.TaskSchedule, error)
	CreateByTaskAndUserID(ctx context.Context, userID domain.UserID, schedule domain.TaskSchedule) (dao.TaskSchedule, error)
	CreateOccurrenceByTaskAndUserID(ctx context.Context, userID domain.UserID, schedule domain.TaskSchedule) (dao.TaskSchedule, error)
	Update(ctx context.Context, schedule domain.TaskSchedule) (dao.TaskSchedule, error)
	UpdateByTaskAndUserID(ctx context.Context, userID domain.UserID, schedule domain.TaskSchedule) (dao.TaskSchedule, error)
	SetCompletedForOwnedTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TaskScheduleID, completed bool) error
	Delete(ctx context.Context, id domain.TaskScheduleID) error
	DeleteByTaskAndUserID(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TaskScheduleID) error
	TombstoneByTaskAndUserID(ctx context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TaskScheduleID) error
	DeleteUneditedFutureBySeries(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.TaskScheduleID, fromAt time.Time) error
	DeleteUneditedFutureByTask(ctx context.Context, userID domain.UserID, taskID domain.TaskID, fromAt time.Time) error
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
