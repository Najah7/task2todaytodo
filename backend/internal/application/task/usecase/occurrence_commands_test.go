package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

type occurrenceRowsRepo struct {
	TodoItemRepository
	rows    []dao.TodoItem
	created *domain.TodoItem
}

type commandAwareTodoRepository struct {
	occurrenceRowsRepo
	listCapability shared.Capability
	getCapability  shared.Capability
}

func (r *commandAwareTodoRepository) ListByTaskForOccurrenceCommand(_ context.Context, _ domain.UserID, _ domain.TaskID, capability shared.Capability) ([]dao.TodoItem, error) {
	r.listCapability = capability
	return r.rows, nil
}

func (r *commandAwareTodoRepository) GetForCommand(_ context.Context, _ domain.UserID, _ domain.TaskID, id domain.TodoItemID, capability shared.Capability) (dao.TodoItem, error) {
	r.getCapability = capability
	for _, row := range r.rows {
		if row.ID == string(id) {
			return row, nil
		}
	}
	return dao.TodoItem{}, ErrTodoItemNotFound
}

func (r occurrenceRowsRepo) ListByTask(context.Context, domain.UserID, domain.TaskID) ([]dao.TodoItem, error) {
	return r.rows, nil
}
func (r *occurrenceRowsRepo) UpsertTodoItemOverride(_ context.Context, _ domain.UserID, item domain.TodoItem) (string, error) {
	r.created = &item
	return string(item.ID), nil
}

type occurrenceRowsRepositories struct {
	taskProgressTestRepositories
	items TodoItemRepository
}

func (r occurrenceRowsRepositories) TodoItems() TodoItemRepository { return r.items }

type occurrenceCommandsUOW struct{ repos Repositories }

func (u occurrenceCommandsUOW) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	return fn(ctx, u.repos)
}

type doneProjectOccurrenceLifecycle struct{}

func (doneProjectOccurrenceLifecycle) LockParent(context.Context, string) error { return nil }
func (doneProjectOccurrenceLifecycle) ReadStatus(context.Context, string) (string, error) {
	return "done", nil
}
func (doneProjectOccurrenceLifecycle) CaptureWorkState(context.Context, string, time.Time) (shared.ProjectWorkState, error) {
	return shared.ProjectWorkState{Status: "done"}, nil
}
func (doneProjectOccurrenceLifecycle) ReconcileWorkState(context.Context, string, string, shared.ProjectWorkState, time.Time) error {
	return nil
}

type projectScopedOccurrenceTaskRepository struct{ taskProgressTestRepository }

func (projectScopedOccurrenceTaskRepository) GetByUserIDWithPermission(_ context.Context, userID domain.UserID, taskID domain.TaskID, _ shared.Capability) (dao.Task, error) {
	return dao.Task{ID: string(taskID), UserID: string(userID), ProjectID: "project", Status: dao.TaskStatus{Value: "open"}}, nil
}

func (projectScopedOccurrenceTaskRepository) LockByUserIDWithPermission(_ context.Context, userID domain.UserID, taskID domain.TaskID, _ shared.Capability) (dao.Task, error) {
	return dao.Task{ID: string(taskID), UserID: string(userID), ProjectID: "project", Status: dao.TaskStatus{Value: "open"}}, nil
}

type doneProjectOccurrenceRepositories struct {
	occurrenceRowsRepositories
	tasks     TaskRepository
	lifecycle shared.ProjectWorkLifecycle
}

func (repos doneProjectOccurrenceRepositories) Tasks() TaskRepository { return repos.tasks }
func (repos doneProjectOccurrenceRepositories) ProjectLifecycle() shared.ProjectWorkLifecycle {
	return repos.lifecycle
}

type skippedOccurrenceTodoRepository struct {
	occurrenceRowsRepo
	skipped []int64
	writes  int
}

func (repo *skippedOccurrenceTodoRepository) ListTodoItemSkippedOccurrences(context.Context, domain.UserID, domain.TaskID, domain.TodoItemID) ([]int64, error) {
	return repo.skipped, nil
}

func (repo *skippedOccurrenceTodoRepository) SetTodoItemSkippedOccurrence(_ context.Context, _ domain.UserID, _ domain.TaskID, _ domain.TodoItemID, occurrenceDate time.Time, skipped bool) error {
	repo.writes++
	if skipped {
		repo.skipped = append(repo.skipped, occurrenceDate.Unix())
		return nil
	}
	filtered := repo.skipped[:0]
	for _, value := range repo.skipped {
		if time.Unix(value, 0).UTC().Format("2006-01-02") != occurrenceDate.UTC().Format("2006-01-02") {
			filtered = append(filtered, value)
		}
	}
	repo.skipped = filtered
	return nil
}

func TestLoadTodoOccurrenceKeepsRootSnapshotAddressableWithoutTreatingItAsChild(t *testing.T) {
	root := recurringTodoRoot()
	root.OccurrenceDate, root.FrequencyAnchorDate = "2026-10-19", mustParseDate("2026-10-19").Unix()
	repos := occurrenceRowsRepositories{items: occurrenceRowsRepo{rows: []dao.TodoItem{root}}}
	state, err := loadTodoOccurrence(context.Background(), repos, "user", "task", "series", root.OccurrenceDate, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if state.current != nil || state.root.ID != "series" || state.date.Format("2006-01-02") != root.OccurrenceDate {
		t.Fatalf("root occurrence state=%#v", state)
	}
}

func TestTodoCommandReadersReceiveTheOperationCapability(t *testing.T) {
	root := recurringTodoRoot()
	repo := &commandAwareTodoRepository{occurrenceRowsRepo: occurrenceRowsRepo{rows: []dao.TodoItem{root}}}
	repos := occurrenceRowsRepositories{items: repo}
	capability := shared.TodoItemUpdate()

	asOf := time.Date(2026, time.October, 5, 12, 0, 0, 0, time.UTC)
	if _, err := loadTodoOccurrenceWithCapability(context.Background(), repos, "actor", "task", domain.TodoItemID(root.ID), root.OccurrenceDate, asOf, capability); err != nil {
		t.Fatalf("loadTodoOccurrenceWithCapability() error = %v", err)
	}
	if repo.listCapability != capability {
		t.Errorf("projection capability = %#v, want %#v", repo.listCapability, capability)
	}
	if _, err := getTodoItemForCommand(context.Background(), repo, "actor", "task", domain.TodoItemID(root.ID), capability); err != nil {
		t.Fatalf("getTodoItemForCommand() error = %v", err)
	}
	if repo.getCapability != capability {
		t.Errorf("get capability = %#v, want %#v", repo.getCapability, capability)
	}
}

func TestDoneProjectBlocksNewTodoSkipButAllowsRestoringSavedSkip(t *testing.T) {
	today := time.Now().UTC().Truncate(24 * time.Hour)
	daysSinceMonday := (int(today.Weekday()) + 6) % 7
	rootDate := today.AddDate(0, 0, -daysSinceMonday)
	virtualDate := rootDate.AddDate(0, 0, 7)
	root := recurringTodoRoot()
	root.OccurrenceDate = rootDate.Format("2006-01-02")
	root.FrequencyAnchorDate = rootDate.Unix()
	repo := &skippedOccurrenceTodoRepository{
		occurrenceRowsRepo: occurrenceRowsRepo{rows: []dao.TodoItem{root}},
	}
	repos := doneProjectOccurrenceRepositories{
		occurrenceRowsRepositories: occurrenceRowsRepositories{items: repo},
		tasks:                      projectScopedOccurrenceTaskRepository{},
		lifecycle:                  doneProjectOccurrenceLifecycle{},
	}
	uow := occurrenceCommandsUOW{repos: repos}

	err := NewSkipTodoItemUseCase(uow, nil).Execute(context.Background(), "user", "task", domain.TodoItemID(root.ID), virtualDate.Format("2006-01-02"))
	if !errors.Is(err, ErrOccurrenceInactive) {
		t.Fatalf("skip unsaved virtual occurrence under done Project = %v; want ErrOccurrenceInactive", err)
	}
	if repo.writes != 0 {
		t.Fatalf("blocked skip made %d writes; want no tombstone", repo.writes)
	}

	repo.skipped = []int64{virtualDate.Unix()}
	err = NewRestoreTodoItemUseCase(uow, nil).Execute(context.Background(), "user", "task", domain.TodoItemID(root.ID), virtualDate.Format("2006-01-02"))
	if err != nil {
		t.Fatalf("restore previously saved skip under done Project: %v", err)
	}
	if repo.writes != 1 || len(repo.skipped) != 0 {
		t.Fatalf("restore writes=%d remaining tombstones=%v; want one restore and no tombstone", repo.writes, repo.skipped)
	}
}

func TestDoneProjectTreatsReopenOfUnsavedTodoVirtualAsInactive(t *testing.T) {
	rootDate, futureDate := nextWeeklyOccurrenceTestDates()
	root := recurringTodoRoot()
	root.OccurrenceDate = rootDate.Format("2006-01-02")
	root.FrequencyAnchorDate = rootDate.Unix()
	repo := occurrenceRowsRepo{rows: []dao.TodoItem{root}}
	repos := doneProjectOccurrenceRepositories{
		occurrenceRowsRepositories: occurrenceRowsRepositories{items: repo},
		tasks:                      projectScopedOccurrenceTaskRepository{},
		lifecycle:                  doneProjectOccurrenceLifecycle{},
	}
	err := NewReopenTodoItemUseCase(occurrenceCommandsUOW{repos: repos}, nil).ExecuteOccurrence(
		context.Background(), "user", "task", domain.TodoItemID(root.ID), futureDate.Format("2006-01-02"),
	)
	if !errors.Is(err, ErrOccurrenceInactive) {
		t.Fatalf("reopen unsaved virtual occurrence under done Project = %v, want ErrOccurrenceInactive", err)
	}
}

func TestCompleteFirstRecurringOccurrenceCreatesChildInsteadOfChangingRoot(t *testing.T) {
	root := recurringTodoRoot()
	root.OccurrenceDate, root.FrequencyAnchorDate = "2026-10-19", mustParseDate("2026-10-19").Unix()
	items := &occurrenceRowsRepo{rows: []dao.TodoItem{root}}
	repos := occurrenceRowsRepositories{items: items}
	uc := NewCompleteTodoItemUseCase(occurrenceCommandsUOW{repos: repos}, nil, fixedUpdateID("first-child-id"))
	if err := uc.ExecuteOccurrence(context.Background(), "user", "task", "series", root.OccurrenceDate); err != nil {
		t.Fatal(err)
	}
	if items.created == nil || items.created.ID != "first-child-id" || string(items.created.ID) == root.ID || !items.created.Completed || !items.created.IsException {
		t.Fatalf("created first occurrence=%#v", items.created)
	}
}

func TestLoadTodoOccurrencePrefersChildAtRootDateRegardlessOfRowOrder(t *testing.T) {
	root := recurringTodoRoot()
	root.OccurrenceDate, root.FrequencyAnchorDate = "2026-10-19", mustParseDate("2026-10-19").Unix()
	child := root
	child.ID = "child"
	child.IsException = true
	child.Title = "edited"
	for _, rows := range [][]dao.TodoItem{{root, child}, {child, root}} {
		repos := occurrenceRowsRepositories{items: occurrenceRowsRepo{rows: rows}}
		state, err := loadTodoOccurrence(context.Background(), repos, "user", "task", "series", root.OccurrenceDate, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if state.current == nil || state.current.ID != "child" || state.current.Title != "edited" {
			t.Fatalf("rows=%#v state=%#v", rows, state)
		}
	}
}

type deletedOccurrenceTaskRepository struct{ TaskRepository }

func (deletedOccurrenceTaskRepository) GetByUserIDWithPermission(_ context.Context, userID domain.UserID, taskID domain.TaskID, _ shared.Capability) (dao.Task, error) {
	return dao.Task{ID: string(taskID), UserID: string(userID), Status: dao.TaskStatus{Value: "open"}}, nil
}

func (deletedOccurrenceTaskRepository) LockByUserIDWithPermission(_ context.Context, userID domain.UserID, taskID domain.TaskID, _ shared.Capability) (dao.Task, error) {
	return dao.Task{ID: string(taskID), UserID: string(userID), Status: dao.TaskStatus{Value: "open"}}, nil
}

func (deletedOccurrenceTaskRepository) ReadTaskProgressSources(_ context.Context, taskIDs []string, _ time.Time) (dao.TaskProgressSources, error) {
	sources := dao.TaskProgressSources{Counts: make(map[string]dao.TaskProgressCounts), Statuses: make(map[string]dao.TaskStatus)}
	for _, taskID := range taskIDs {
		sources.Statuses[taskID] = dao.TaskStatus{Value: "open"}
	}
	return sources, nil
}

func (deletedOccurrenceTaskRepository) SetStatusByUserIDWithPermission(context.Context, domain.UserID, domain.TaskID, domain.TaskStatus, int32, shared.Capability) error {
	return nil
}

type deletedOccurrenceTodoRepository struct {
	TodoItemRepository
	rows          []dao.TodoItem
	mutationCalls int
}

func (r *deletedOccurrenceTodoRepository) ListByTask(context.Context, domain.UserID, domain.TaskID) ([]dao.TodoItem, error) {
	return r.rows, nil
}

func (*deletedOccurrenceTodoRepository) ListTodoItemSkippedOccurrences(context.Context, domain.UserID, domain.TaskID, domain.TodoItemID) ([]int64, error) {
	return nil, nil
}

func (r *deletedOccurrenceTodoRepository) SetTodoItemSkippedOccurrence(context.Context, domain.UserID, domain.TaskID, domain.TodoItemID, time.Time, bool) error {
	r.mutationCalls++
	return nil
}

func (r *deletedOccurrenceTodoRepository) CheckForOwnedTask(context.Context, domain.UserID, domain.TaskID, domain.TodoItemID) error {
	r.mutationCalls++
	return nil
}

func (r *deletedOccurrenceTodoRepository) UpsertTodoItemOverride(_ context.Context, _ domain.UserID, item domain.TodoItem) (string, error) {
	r.mutationCalls++
	return string(item.ID), nil
}

type deletedOccurrenceRepositories struct {
	taskProgressTestRepositories
	tasks     TaskRepository
	todoItems TodoItemRepository
}

func (r deletedOccurrenceRepositories) Tasks() TaskRepository         { return r.tasks }
func (r deletedOccurrenceRepositories) TodoItems() TodoItemRepository { return r.todoItems }

func nextWeeklyOccurrenceTestDates() (time.Time, time.Time) {
	today := time.Now().UTC()
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	daysUntilMonday := (8 - int(today.Weekday())) % 7
	if daysUntilMonday == 0 {
		daysUntilMonday = 7
	}
	futureDate := today.AddDate(0, 0, daysUntilMonday)
	return futureDate.AddDate(0, 0, -7), futureDate
}

func TestDeletedTodoOccurrenceRejectsRestoreCompleteAndEdit(t *testing.T) {
	rootDate, futureDate := nextWeeklyOccurrenceTestDates()
	root := recurringTodoRoot()
	root.OccurrenceDate, root.FrequencyAnchorDate = rootDate.Format("2006-01-02"), rootDate.Unix()
	deletedFuture := root
	deletedFuture.ID, deletedFuture.OccurrenceDate, deletedFuture.Deleted, deletedFuture.IsException = "deleted-future", futureDate.Format("2006-01-02"), true, true
	deletedRootDate := root
	deletedRootDate.ID, deletedRootDate.Deleted, deletedRootDate.IsException = "deleted-root-date", true, true
	tests := []struct {
		name string
		date string
		rows []dao.TodoItem
	}{
		{name: "deleted future occurrence", date: deletedFuture.OccurrenceDate, rows: []dao.TodoItem{root, deletedFuture}},
		{name: "deleted saved occurrence on root date", date: root.OccurrenceDate, rows: []dao.TodoItem{root, deletedRootDate}},
	}
	operations := []struct {
		name string
		run  func(context.Context, *occurrenceCommandsUOW, string) error
	}{
		{name: "restore", run: func(ctx context.Context, uow *occurrenceCommandsUOW, date string) error {
			return NewRestoreTodoItemUseCase(uow, nil).Execute(ctx, "user", "task", "series", date)
		}},
		{name: "complete", run: func(ctx context.Context, uow *occurrenceCommandsUOW, date string) error {
			return NewCompleteTodoItemUseCase(uow, nil, fixedUpdateID("new-todo")).ExecuteOccurrence(ctx, "user", "task", "series", date)
		}},
		{name: "edit", run: func(ctx context.Context, uow *occurrenceCommandsUOW, date string) error {
			title := "changed"
			_, err := NewUpdateTodoItemUseCase(uow, nil, fixedUpdateID("new-todo")).ExecuteOccurrence(ctx, "user", "task", "series", date, todoItemScopeCurrent, PatchField[string]{Present: true, Value: &title}, PatchField[string]{}, PatchField[time.Time]{})
			return err
		}},
	}
	for _, occurrence := range tests {
		for _, operation := range operations {
			t.Run(occurrence.name+"/"+operation.name, func(t *testing.T) {
				repo := &deletedOccurrenceTodoRepository{rows: occurrence.rows}
				repos := deletedOccurrenceRepositories{tasks: deletedOccurrenceTaskRepository{}, todoItems: repo}
				uow := &occurrenceCommandsUOW{repos: repos}
				if err := operation.run(context.Background(), uow, occurrence.date); !errors.Is(err, ErrOccurrenceInactive) {
					t.Fatalf("operation error = %v; want ErrOccurrenceInactive", err)
				}
				if repo.mutationCalls != 0 {
					t.Fatalf("deleted occurrence triggered %d writes", repo.mutationCalls)
				}
			})
		}
	}
}
