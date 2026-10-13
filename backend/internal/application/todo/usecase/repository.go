package usecase

import (
	"context"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/todo/dao"
	"github.com/Najah7/task2todaytodo/internal/application/todo/domain"
)

type UOW interface {
	Do(context.Context, func(context.Context, Repositories) error) error
}

type Repositories interface {
	Todos() TodoRepository
	Sources() SourceReader
	Items() ItemRepository
	ActionItems() ActionItemCommands
	Tasks() TaskCommands
	Schedules() ScheduleCommands
}

// ProjectWriteAccessRepository reads sharing and effective Team entitlements
// from the operation's transaction. It is intentionally discovered as an
// optional extension so personal-only test repositories remain small; shared
// project writes fail closed when the extension is unavailable.
type ProjectWriteAccessRepository interface {
	ProjectWriteAccess() shared.ProjectWriteAccessReader
}

// TodoRepository persists todos and their ordered daily rows. All methods are
// bound to the UOW transaction supplied by the application root.
type TodoRepository interface {
	LockUser(context.Context, string) error
	GetByUserDate(context.Context, string, time.Time) (dao.Todo, error)
	Create(context.Context, domain.Todo) (dao.Todo, error)
	BumpRevision(context.Context, string, int32) (int32, error)
	UpdateTimezoneAndBumpRevision(context.Context, domain.Todo, int32) (int32, error)
}

type ItemRepository interface {
	ListByTodoID(context.Context, string) ([]dao.Item, error)
	Insert(context.Context, domain.TodoItem) (dao.Item, error)
	DeleteAllByTodoID(context.Context, string) error
	Save(context.Context, domain.TodoItem) (dao.Item, error)
	SavePositions(context.Context, string, []domain.TodoItem) error
	DeleteByID(context.Context, string, string) error
	HasTodayWork(context.Context, string, dao.WorkSourceRef) (bool, error)
	HasCompletedWork(context.Context, dao.WorkSourceRef) (bool, error)
	HasCompletedTaskFrame(context.Context, string) (bool, error)
	HasTodayTaskFrame(context.Context, string, string) (bool, error)
	HasTodayTaskWork(context.Context, string, string) (bool, error)
	CompleteTodayTaskFrames(context.Context, string, string) error
	ReopenTaskFrames(context.Context, string) error
	CompleteTodayFrames(context.Context, string, string) error
	ReopenFrames(context.Context, string) error
}

// SourceReader is a Todo-owned projection port. It must enforce the caller's
// ActionItem read ACL and Task assignment and return Schedule identities as
// rule_id + original occurrence_date, including virtual occurrences.
type SourceReader interface {
	CurrentDay(context.Context, string) (dao.CurrentDay, error)
	// Planning and work sources are resolved in the persisted/generated todo's
	// calendar, so callers always supply that todo's date and timezone.
	ReadPlanningSources(context.Context, string, time.Time, string) (dao.PlanningSources, error)
	ReadSavedScheduleSources(context.Context, string, time.Time, string) ([]dao.ScheduleOccurrence, error)
	ReadWorkSources(context.Context, string, time.Time, string, []dao.WorkSourceRef) ([]dao.WorkSource, error)
}

// ActionItemCommands delegates whole-source completion to the owning Task
// context. Todo frame completion is a different operation and never mutates
// ActionItem actual_minutes itself.
type ActionItemCommands interface {
	CompleteFromTodoFrame(context.Context, string, string, string, *int) error
	ReopenFromTodoFrame(context.Context, string, string, string) error
}

type TaskCommands interface {
	CompleteFromTodoFrame(context.Context, string, string, *int) error
	ReopenFromTodoFrame(context.Context, string, string) error
}

// ScheduleCommands delegates occurrence completion to the Schedule owner.
// The stable source key is rule ID + original occurrence date.
type ScheduleCommands interface {
	Complete(context.Context, string, string, time.Time, int32) error
	Reopen(context.Context, string, string, time.Time, int32) error
}

// ActionItemTodoLifecycle is the primitive reverse bridge required by Task
// commands. Root binds it to the same transaction as the source mutation.
type ActionItemTodoLifecycle interface {
	HasCompletedFrame(context.Context, string) (bool, error)
	HasTodayActionItem(context.Context, string, string) (bool, error)
	CompleteTodayFrames(context.Context, string, string) error
	ReopenFrames(context.Context, string) error
}

type TaskTodoLifecycle interface {
	HasCompletedTaskFrame(context.Context, string) (bool, error)
	HasTodayTaskFrame(context.Context, string, string) (bool, error)
	HasTodayTaskWork(context.Context, string, string) (bool, error)
	CompleteTodayTaskFrames(context.Context, string, string) error
	ReopenTaskFrames(context.Context, string) error
}
