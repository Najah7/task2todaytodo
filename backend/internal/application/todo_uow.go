package application

import (
	"context"
	"errors"
	"time"

	allocationrepository "github.com/Najah7/task2todaytodo/internal/application/allocation/repository"
	projectusecase "github.com/Najah7/task2todaytodo/internal/application/project/usecase"
	scheduleusecase "github.com/Najah7/task2todaytodo/internal/application/schedule/usecase"
	taskdomain "github.com/Najah7/task2todaytodo/internal/application/task/domain"
	taskusecase "github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	tododao "github.com/Najah7/task2todaytodo/internal/application/todo/dao"
	todorepository "github.com/Najah7/task2todaytodo/internal/application/todo/repository"
	todousecase "github.com/Najah7/task2todaytodo/internal/application/todo/usecase"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
)

type TodoUOW struct {
	pool      *pgxpool.Pool
	store     Store
	tasks     TaskStore
	projects  ProjectStore
	schedules ScheduleStore
	newID     func() string
	logger    telemetry.Logger
}

func NewTodoUOW(pool *pgxpool.Pool, store Store, tasks TaskStore, projects ProjectStore, schedules ScheduleStore, newID func() string, logger telemetry.Logger) todousecase.UOW {
	if newID == nil {
		newID = func() string { return ulid.Make().String() }
	}
	return &TodoUOW{pool: pool, store: store, tasks: tasks, projects: projects, schedules: schedules, newID: newID, logger: telemetry.OrNop(logger)}
}

func (u *TodoUOW) Do(ctx context.Context, fn func(context.Context, todousecase.Repositories) error) error {
	return RunInTodoWorkflowTx(ctx, u.pool, func(tx pgx.Tx) error {
		return fn(ctx, u.repositories(tx))
	})
}

func (u *TodoUOW) repositories(tx pgx.Tx) todousecase.Repositories {
	return newTodoRepositories(u.store, u.tasks, u.projects, u.schedules, tx, u.newID, u.logger)
}

func newTodoRepositories(store Store, taskStore TaskStore, projectStore ProjectStore, scheduleStore ScheduleStore, tx pgx.Tx, newID func() string, logger telemetry.Logger) todousecase.Repositories {
	return newTodoRepositoriesWithSource(NewTodoSourceReader(store, tx), taskStore, projectStore, scheduleStore, tx, newID, logger)
}

func newTodoRepositoriesFromTx(taskStore TaskStore, projectStore ProjectStore, scheduleStore ScheduleStore, tx pgx.Tx, newID func() string, logger telemetry.Logger) todousecase.Repositories {
	return newTodoRepositoriesWithSource(NewTodoSourceReaderFromTx(tx), taskStore, projectStore, scheduleStore, tx, newID, logger)
}

func newTodoRepositoriesWithSource(sourceReader todousecase.SourceReader, taskStore TaskStore, projectStore ProjectStore, scheduleStore ScheduleStore, tx pgx.Tx, newID func() string, logger telemetry.Logger) todousecase.Repositories {
	items := todorepository.NewItemRepository(tx)
	todos := todorepository.NewTodoRepository(tx)

	projects := projectStore.WithTx(tx).Projects
	tasks := taskStore.WithTx(tx)
	scheduleRows := scheduleStore.WithTx(tx).Schedules
	allocationRows := allocationrepository.NewAllocationRepositoryFromTx(tx)
	taskRepos := taskRepositories{
		store: tasks,
		lifecycle: projectusecase.NewProjectLifecycleUseCase(projects, projects, scheduleRows,
			allocationProjectLifecycleAdapter{repository: allocationRows}),
	}
	// The Todo owner command deliberately does not invoke Task's reverse Todo
	// bridge. The selected Todo frame is already mutated in this transaction.
	taskCommands := todoActionItemCommands{
		complete: taskusecase.NewCompleteActionItemUseCase(nil, logger),
		reopen:   taskusecase.NewReopenActionItemUseCase(nil, logger),
		repos:    taskRepos,
	}
	wholeTaskCommands := todoTaskCommands{
		complete: taskusecase.NewCompleteTaskUseCase(nil, time.Now, logger),
		reopen:   taskusecase.NewReopenTaskUseCase(nil, logger),
		repos:    taskRepos,
	}
	scheduleRepos := scheduleRepositories{
		store: scheduleStore.WithTx(tx),
		lifecycle: projectusecase.NewProjectLifecycleUseCase(projects, projects, scheduleRows,
			allocationProjectLifecycleAdapter{repository: allocationRows}),
	}
	scheduleCommands := todoScheduleCommands{repos: scheduleRepos, newID: newID}
	return todoRepositories{todos: todos, items: items, sources: sourceReader, actionItems: taskCommands, tasks: wholeTaskCommands, schedules: scheduleCommands}
}

type todoRepositories struct {
	todos       todousecase.TodoRepository
	items       todousecase.ItemRepository
	sources     todousecase.SourceReader
	actionItems todousecase.ActionItemCommands
	tasks       todousecase.TaskCommands
	schedules   todousecase.ScheduleCommands
}

func (r todoRepositories) Todos() todousecase.TodoRepository           { return r.todos }
func (r todoRepositories) Sources() todousecase.SourceReader           { return r.sources }
func (r todoRepositories) Items() todousecase.ItemRepository           { return r.items }
func (r todoRepositories) ActionItems() todousecase.ActionItemCommands { return r.actionItems }
func (r todoRepositories) Tasks() todousecase.TaskCommands             { return r.tasks }
func (r todoRepositories) Schedules() todousecase.ScheduleCommands     { return r.schedules }

// todoActionItemCommands delegates selected-frame completion to the Task
// owner without running the reverse all-today-frame synchronization.
type todoActionItemCommands struct {
	complete *taskusecase.CompleteActionItemUseCase
	reopen   *taskusecase.ReopenActionItemUseCase
	repos    taskusecase.Repositories
}

func (c todoActionItemCommands) CompleteFromTodoFrame(ctx context.Context, actorID, taskID, actionItemID string, actual *int) error {
	err := c.complete.CompleteInTx(ctx, c.repos, taskdomain.UserID(actorID), taskdomain.TaskID(taskID), taskdomain.ActionItemID(actionItemID), actual, false)
	return mapTodoTaskOwnerError(err)
}

func (c todoActionItemCommands) ReopenFromTodoFrame(ctx context.Context, actorID, taskID, actionItemID string) error {
	err := c.reopen.ReopenInTx(ctx, c.repos, taskdomain.UserID(actorID), taskdomain.TaskID(taskID), taskdomain.ActionItemID(actionItemID), false)
	return mapTodoTaskOwnerError(err)
}

type todoTaskCommands struct {
	complete *taskusecase.CompleteTaskUseCase
	reopen   *taskusecase.ReopenTaskUseCase
	repos    taskusecase.Repositories
}

func (c todoTaskCommands) CompleteFromTodoFrame(ctx context.Context, actorID, taskID string, actual *int) error {
	err := c.complete.CompleteFromTodoFrame(ctx, c.repos, actorID, taskID, actual)
	return mapTodoTaskOwnerError(err)
}
func (c todoTaskCommands) ReopenFromTodoFrame(ctx context.Context, actorID, taskID string) error {
	err := c.reopen.ReopenFromTodoFrame(ctx, c.repos, actorID, taskID)
	return mapTodoTaskOwnerError(err)
}

func mapTodoTaskOwnerError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, taskusecase.ErrActionItemNotFound), errors.Is(err, taskusecase.ErrActionItemTaskNotFound), errors.Is(err, taskusecase.ErrTaskNotFound):
		return todousecase.ErrActionItemNotFound
	case errors.Is(err, taskusecase.ErrPermissionDenied):
		return todousecase.ErrPermissionDenied
	case errors.Is(err, taskusecase.ErrRevisionConflict):
		return todousecase.ErrRevisionConflict
	case errors.Is(err, taskusecase.ErrActionItemStarted):
		return todousecase.ErrActionItemStarted
	default:
		return err
	}
}

type todoScheduleCommands struct {
	repos scheduleusecase.Repositories
	newID func() string
}

func (c todoScheduleCommands) Complete(ctx context.Context, actorID, ruleID string, occurrenceDate time.Time, revision int32) error {
	err := scheduleusecase.NewSetOccurrenceStateUseCase(nil, "complete", nil, c.newID).ExecuteInTx(
		ctx, actorID, ruleID, occurrenceDate.Format("2006-01-02"), revision, c.repos)
	return mapTodoScheduleOwnerError(err)
}

func (c todoScheduleCommands) Reopen(ctx context.Context, actorID, ruleID string, occurrenceDate time.Time, revision int32) error {
	err := scheduleusecase.NewSetOccurrenceStateUseCase(nil, "reopen", nil, c.newID).ExecuteInTx(
		ctx, actorID, ruleID, occurrenceDate.Format("2006-01-02"), revision, c.repos)
	return mapTodoScheduleOwnerError(err)
}

func mapTodoScheduleOwnerError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, scheduleusecase.ErrOccurrenceNotFound), errors.Is(err, scheduleusecase.ErrOccurrenceInactive):
		return todousecase.ErrFrameNotFound
	case errors.Is(err, scheduleusecase.ErrPermissionDenied):
		return todousecase.ErrPermissionDenied
	case errors.Is(err, scheduleusecase.ErrOccurrenceRevisionMismatch):
		return todousecase.ErrRevisionConflict
	case errors.Is(err, scheduleusecase.ErrOccurrenceCompleted):
		return todousecase.ErrScheduleCompleted
	default:
		return err
	}
}

type taskTodoLifecycle struct {
	items todousecase.ItemRepository
}

func (l taskTodoLifecycle) HasCompletedFrame(ctx context.Context, actionItemID taskdomain.ActionItemID) (bool, error) {
	return l.items.HasCompletedWork(ctx, tododao.WorkSourceRef{Kind: "action_item", ID: string(actionItemID)})
}
func (l taskTodoLifecycle) HasTodayActionItem(ctx context.Context, userID taskdomain.UserID, actionItemID taskdomain.ActionItemID) (bool, error) {
	return l.items.HasTodayWork(ctx, string(userID), tododao.WorkSourceRef{Kind: "action_item", ID: string(actionItemID)})
}
func (l taskTodoLifecycle) CompleteTodayFrames(ctx context.Context, userID taskdomain.UserID, actionItemID taskdomain.ActionItemID) error {
	return l.items.CompleteTodayFrames(ctx, string(userID), string(actionItemID))
}
func (l taskTodoLifecycle) ReopenFrames(ctx context.Context, actionItemID taskdomain.ActionItemID) error {
	return l.items.ReopenFrames(ctx, string(actionItemID))
}
func (l taskTodoLifecycle) HasCompletedTaskFrame(ctx context.Context, taskID taskdomain.TaskID) (bool, error) {
	return l.items.HasCompletedTaskFrame(ctx, string(taskID))
}
func (l taskTodoLifecycle) HasTodayTaskFrame(ctx context.Context, userID taskdomain.UserID, taskID taskdomain.TaskID) (bool, error) {
	return l.items.HasTodayTaskFrame(ctx, string(userID), string(taskID))
}
func (l taskTodoLifecycle) HasTodayTaskWork(ctx context.Context, userID taskdomain.UserID, taskID taskdomain.TaskID) (bool, error) {
	return l.items.HasTodayTaskWork(ctx, string(userID), string(taskID))
}
func (l taskTodoLifecycle) CompleteTodayTaskFrames(ctx context.Context, userID taskdomain.UserID, taskID taskdomain.TaskID) error {
	return l.items.CompleteTodayTaskFrames(ctx, string(userID), string(taskID))
}
func (l taskTodoLifecycle) ReopenTaskFrames(ctx context.Context, taskID taskdomain.TaskID) error {
	return l.items.ReopenTaskFrames(ctx, string(taskID))
}
