package usecase

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

type listActionItemsUOWFake struct {
	repos Repositories
	err   error
	calls int
}

func (uow *listActionItemsUOWFake) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	uow.calls++
	if uow.err != nil {
		return uow.err
	}
	return fn(ctx, uow.repos)
}

type listActionItemsRepositoriesFake struct {
	taskProgressTestRepositories
	tasks       TaskRepository
	actionItems ActionItemRepository
	lifecycle   shared.ProjectWorkLifecycle
	accesses    *[]string
}

func (repos listActionItemsRepositoriesFake) Tasks() TaskRepository {
	*repos.accesses = append(*repos.accesses, "tasks")
	return repos.tasks
}

func (repos listActionItemsRepositoriesFake) ActionItems() ActionItemRepository {
	*repos.accesses = append(*repos.accesses, "action-items")
	return repos.actionItems
}

func (repos listActionItemsRepositoriesFake) ProjectLifecycle() shared.ProjectWorkLifecycle {
	if repos.lifecycle != nil {
		return repos.lifecycle
	}
	return repos.taskProgressTestRepositories.ProjectLifecycle()
}

type listActionItemsProjectLifecycleFake struct {
	status       string
	readCalls    int
	captureCalls int
}

func (*listActionItemsProjectLifecycleFake) LockParent(context.Context, string) error { return nil }
func (f *listActionItemsProjectLifecycleFake) ReadStatus(context.Context, string) (string, error) {
	f.readCalls++
	return f.status, nil
}
func (f *listActionItemsProjectLifecycleFake) CaptureWorkState(context.Context, string, time.Time) (shared.ProjectWorkState, error) {
	f.captureCalls++
	return shared.ProjectWorkState{Status: f.status}, nil
}
func (*listActionItemsProjectLifecycleFake) ReconcileWorkState(context.Context, string, string, shared.ProjectWorkState, time.Time) error {
	return nil
}

type listActionItemsTaskRepositoryFake struct {
	taskProgressTestRepository
	task       dao.Task
	err        error
	userID     domain.UserID
	taskID     domain.TaskID
	permission shared.Capability
	calls      int
	accesses   *[]string
}

func (repo *listActionItemsTaskRepositoryFake) GetByUserIDWithPermission(_ context.Context, userID domain.UserID, taskID domain.TaskID, permission shared.Capability) (dao.Task, error) {
	repo.calls++
	repo.userID, repo.taskID = userID, taskID
	repo.permission = permission
	*repo.accesses = append(*repo.accesses, "get-task")
	if repo.err != nil {
		return dao.Task{}, repo.err
	}
	return repo.task, nil
}

type listActionItemsRepositoryFake struct {
	ActionItemRepository
	items    []dao.ActionItem
	err      error
	userID   domain.UserID
	taskID   domain.TaskID
	calls    int
	accesses *[]string
}

func (repo *listActionItemsRepositoryFake) ListByTask(_ context.Context, userID domain.UserID, taskID domain.TaskID) ([]dao.ActionItem, error) {
	repo.calls++
	repo.userID, repo.taskID = userID, taskID
	*repo.accesses = append(*repo.accesses, "list-action-items")
	return repo.items, repo.err
}

func (repo *listActionItemsRepositoryFake) ListByTaskCursor(_ context.Context, userID domain.UserID, taskID domain.TaskID, limit int, anchor *CursorAnchor) ([]dao.ActionItem, error) {
	repo.calls++
	repo.userID, repo.taskID = userID, taskID
	*repo.accesses = append(*repo.accesses, "list-action-items")
	if repo.err != nil {
		return nil, repo.err
	}
	items := append([]dao.ActionItem(nil), repo.items...)
	sort.Slice(items, func(i, j int) bool {
		if items[i].Position != items[j].Position {
			return items[i].Position < items[j].Position
		}
		return items[i].ID < items[j].ID
	})
	if anchor != nil {
		filtered := items[:0]
		for _, item := range items {
			if item.Position > anchor.Position || item.Position == anchor.Position && item.ID > anchor.ID {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func TestListActionItemsUseCaseExecuteReturnsReadableSharedTaskItemsInRepositoryOrder(t *testing.T) {
	userID := domain.UserID("user-1")
	taskID := domain.TaskID("task-1")
	wantItems := []dao.ActionItem{
		{ID: "item-1", TaskID: string(taskID), Position: 0, Title: "First"},
		{ID: "item-2", TaskID: string(taskID), Position: 1, Title: "Second"},
		{ID: "item-3", TaskID: string(taskID), Position: 2, Title: "Third"},
	}
	var accesses []string
	taskRepo := &listActionItemsTaskRepositoryFake{task: dao.Task{ID: string(taskID), UserID: "owner-1"}, accesses: &accesses}
	itemsRepo := &listActionItemsRepositoryFake{items: wantItems, accesses: &accesses}
	uow := &listActionItemsUOWFake{repos: listActionItemsRepositoriesFake{
		tasks: taskRepo, actionItems: itemsRepo, accesses: &accesses,
	}}

	got, err := NewListActionItemsUseCase(uow, nil).Execute(context.Background(), userID, taskID)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if !reflect.DeepEqual(got, wantItems) {
		t.Errorf("Execute() = %#v, want repository order %#v", got, wantItems)
	}
	if uow.calls != 1 || taskRepo.calls != 1 || itemsRepo.calls != 1 {
		t.Errorf("calls = UOW:%d task:%d items:%d, want 1 each", uow.calls, taskRepo.calls, itemsRepo.calls)
	}
	if taskRepo.userID != userID || taskRepo.taskID != taskID || taskRepo.permission != shared.ActionItemRead() || itemsRepo.userID != userID || itemsRepo.taskID != taskID {
		t.Errorf("repository scope = task(%q,%q), items(%q,%q), want user %q and task %q", taskRepo.userID, taskRepo.taskID, itemsRepo.userID, itemsRepo.taskID, userID, taskID)
	}
	if !reflect.DeepEqual(accesses, []string{"tasks", "get-task", "action-items", "list-action-items"}) {
		t.Errorf("repository access order = %v, want task permission check before item listing", accesses)
	}
}

func TestListActionItemsUseCaseExecuteReturnsNonNilEmptyList(t *testing.T) {
	var accesses []string
	taskRepo := &listActionItemsTaskRepositoryFake{task: dao.Task{UserID: "user-1"}, accesses: &accesses}
	itemsRepo := &listActionItemsRepositoryFake{accesses: &accesses}
	uow := &listActionItemsUOWFake{repos: listActionItemsRepositoriesFake{
		tasks: taskRepo, actionItems: itemsRepo, accesses: &accesses,
	}}

	got, err := NewListActionItemsUseCase(uow, nil).Execute(context.Background(), "user-1", "task-1")
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("Execute() = %#v, want non-nil empty list", got)
	}
}

func TestListActionItemsUseCaseExecuteStopsWhenActionItemReadIsDenied(t *testing.T) {
	userID := domain.UserID("user-1")
	taskID := domain.TaskID("task-1")
	var accesses []string
	taskRepo := &listActionItemsTaskRepositoryFake{err: ErrTaskNotFound, accesses: &accesses}
	itemsRepo := &listActionItemsRepositoryFake{items: []dao.ActionItem{{ID: "must-not-be-read"}}, accesses: &accesses}
	uow := &listActionItemsUOWFake{repos: listActionItemsRepositoriesFake{
		tasks: taskRepo, actionItems: itemsRepo, accesses: &accesses,
	}}

	got, err := NewListActionItemsUseCase(uow, nil).Execute(context.Background(), userID, taskID)
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("Execute() error = %v, want %v", err, ErrTaskNotFound)
	}
	if got != nil {
		t.Errorf("Execute() = %#v, want nil result on error", got)
	}
	if itemsRepo.calls != 0 {
		t.Errorf("ListByTask() calls = %d, want 0 when action_item/read is denied", itemsRepo.calls)
	}
}

func TestListActionItemsUseCaseExecutePropagatesRepositoryErrors(t *testing.T) {
	userID := domain.UserID("user-1")
	taskID := domain.TaskID("task-1")
	wantErr := errors.New("database unavailable")
	t.Run("task repository", func(t *testing.T) {
		var accesses []string
		taskRepo := &listActionItemsTaskRepositoryFake{err: wantErr, accesses: &accesses}
		itemsRepo := &listActionItemsRepositoryFake{accesses: &accesses}
		uow := &listActionItemsUOWFake{repos: listActionItemsRepositoriesFake{
			tasks: taskRepo, actionItems: itemsRepo, accesses: &accesses,
		}}
		got, err := NewListActionItemsUseCase(uow, nil).Execute(context.Background(), userID, taskID)
		if !errors.Is(err, wantErr) || got != nil || itemsRepo.calls != 0 {
			t.Errorf("Execute() = (%#v, %v), item calls %d; want nil result, original error, 0", got, err, itemsRepo.calls)
		}
	})
	t.Run("action item repository", func(t *testing.T) {
		var accesses []string
		taskRepo := &listActionItemsTaskRepositoryFake{task: dao.Task{UserID: string(userID)}, accesses: &accesses}
		itemsRepo := &listActionItemsRepositoryFake{err: wantErr, accesses: &accesses}
		uow := &listActionItemsUOWFake{repos: listActionItemsRepositoriesFake{
			tasks: taskRepo, actionItems: itemsRepo, accesses: &accesses,
		}}
		got, err := NewListActionItemsUseCase(uow, nil).Execute(context.Background(), userID, taskID)
		if !errors.Is(err, wantErr) || got != nil {
			t.Errorf("Execute() = (%#v, %v), want nil result and original error", got, err)
		}
	})
}

func TestListActionItemsUseCaseExecutePropagatesUOWError(t *testing.T) {
	wantErr := errors.New("transaction failed")
	uow := &listActionItemsUOWFake{err: wantErr}

	got, err := NewListActionItemsUseCase(uow, nil).Execute(context.Background(), "user-1", "task-1")
	if !errors.Is(err, wantErr) {
		t.Errorf("Execute() error = %v, want %v", err, wantErr)
	}
	if got != nil {
		t.Errorf("Execute() = %#v, want nil result on error", got)
	}
	if uow.calls != 1 {
		t.Errorf("UOW calls = %d, want 1", uow.calls)
	}
}

func TestListActionItemsPageReturnsPersistedItemsInPositionAndIDOrder(t *testing.T) {
	var accesses []string
	taskRepo := &listActionItemsTaskRepositoryFake{
		task:     dao.Task{ID: "task-1", UserID: "user-1", ProjectID: "project-1"},
		accesses: &accesses,
	}
	itemsRepo := &listActionItemsRepositoryFake{items: []dao.ActionItem{
		{ID: "item-2", TaskID: "task-1", Position: 1, Title: "Second"},
		{ID: "item-1", TaskID: "task-1", Position: 0, Title: "First"},
		{ID: "item-3", TaskID: "task-1", Position: 2, Title: "Third"},
	}, accesses: &accesses}
	lifecycle := &listActionItemsProjectLifecycleFake{status: "done"}
	uow := &listActionItemsUOWFake{repos: listActionItemsRepositoriesFake{
		tasks: taskRepo, actionItems: itemsRepo, lifecycle: lifecycle, accesses: &accesses,
	}}

	page, err := NewListActionItemsUseCase(uow, nil).ExecutePage(context.Background(), "user-1", "task-1", CursorPageRequest{Size: 2})
	if err != nil {
		t.Fatalf("ExecutePage() error = %v", err)
	}
	if lifecycle.readCalls != 0 || lifecycle.captureCalls != 0 {
		t.Fatalf("Project lifecycle calls = ReadStatus:%d CaptureWorkState:%d; want no schedule projection", lifecycle.readCalls, lifecycle.captureCalls)
	}
	if len(page.Items) != 2 || page.Items[0].ID != "item-1" || page.Items[1].ID != "item-2" {
		t.Errorf("ExecutePage() items = %#v, want first two persisted rows ordered by position", page.Items)
	}
	if page.Next == nil || page.Next.Position != 1 || page.Next.ID != "item-2" {
		t.Errorf("ExecutePage() next = %#v, want cursor after second persisted row", page.Next)
	}
}
