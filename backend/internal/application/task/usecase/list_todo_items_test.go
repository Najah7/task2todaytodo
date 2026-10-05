package usecase

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

type listTodoItemsUOWFake struct {
	repos Repositories
	err   error
	calls int
}

func (uow *listTodoItemsUOWFake) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	uow.calls++
	if uow.err != nil {
		return uow.err
	}
	return fn(ctx, uow.repos)
}

type listTodoItemsRepositoriesFake struct {
	taskProgressTestRepositories
	tasks     TaskRepository
	todoItems TodoItemRepository
	accesses  *[]string
}

func (repos listTodoItemsRepositoriesFake) Tasks() TaskRepository {
	*repos.accesses = append(*repos.accesses, "tasks")
	return repos.tasks
}

func (repos listTodoItemsRepositoriesFake) TodoItems() TodoItemRepository {
	*repos.accesses = append(*repos.accesses, "todo-items")
	return repos.todoItems
}

type listTodoItemsTaskRepositoryFake struct {
	taskProgressTestRepository
	task       dao.Task
	err        error
	userID     domain.UserID
	taskID     domain.TaskID
	permission shared.Capability
	calls      int
	accesses   *[]string
}

func (repo *listTodoItemsTaskRepositoryFake) GetByUserIDWithPermission(_ context.Context, userID domain.UserID, taskID domain.TaskID, permission shared.Capability) (dao.Task, error) {
	repo.calls++
	repo.userID, repo.taskID = userID, taskID
	repo.permission = permission
	*repo.accesses = append(*repo.accesses, "get-task")
	if repo.err != nil {
		return dao.Task{}, repo.err
	}
	return repo.task, nil
}

type listTodoItemsRepositoryFake struct {
	TodoItemRepository
	items    []dao.TodoItem
	err      error
	userID   domain.UserID
	taskID   domain.TaskID
	calls    int
	accesses *[]string
}

func (repo *listTodoItemsRepositoryFake) ListByTask(_ context.Context, userID domain.UserID, taskID domain.TaskID) ([]dao.TodoItem, error) {
	repo.calls++
	repo.userID, repo.taskID = userID, taskID
	*repo.accesses = append(*repo.accesses, "list-todo-items")
	return repo.items, repo.err
}

func TestListTodoItemsUseCaseExecuteReturnsReadableSharedTaskItemsInRepositoryOrder(t *testing.T) {
	userID := domain.UserID("user-1")
	taskID := domain.TaskID("task-1")
	wantItems := []dao.TodoItem{
		{ID: "source", TaskID: string(taskID), Position: 1, OccurrenceDate: "2026-10-03"},
		{ID: "generated-1", TaskID: string(taskID), Position: 1, OccurrenceDate: "2026-10-04"},
		{ID: "next", TaskID: string(taskID), Position: 2, OccurrenceDate: "2026-10-03"},
	}
	var accesses []string
	taskRepo := &listTodoItemsTaskRepositoryFake{task: dao.Task{ID: string(taskID), UserID: "owner-1"}, accesses: &accesses}
	itemsRepo := &listTodoItemsRepositoryFake{items: wantItems, accesses: &accesses}
	uow := &listTodoItemsUOWFake{repos: listTodoItemsRepositoriesFake{
		tasks: taskRepo, todoItems: itemsRepo, accesses: &accesses,
	}}

	got, err := NewListTodoItemsUseCase(uow, nil).Execute(context.Background(), userID, taskID)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if !reflect.DeepEqual(got, wantItems) {
		t.Errorf("Execute() = %#v, want repository order %#v", got, wantItems)
	}
	if uow.calls != 1 || taskRepo.calls != 1 || itemsRepo.calls != 1 {
		t.Errorf("calls = UOW:%d task:%d items:%d, want 1 each", uow.calls, taskRepo.calls, itemsRepo.calls)
	}
	if taskRepo.userID != userID || taskRepo.taskID != taskID || taskRepo.permission != shared.TodoItemRead() || itemsRepo.userID != userID || itemsRepo.taskID != taskID {
		t.Errorf("repository scope = task(%q,%q), items(%q,%q), want user %q and task %q", taskRepo.userID, taskRepo.taskID, itemsRepo.userID, itemsRepo.taskID, userID, taskID)
	}
	if !reflect.DeepEqual(accesses, []string{"tasks", "get-task", "todo-items", "list-todo-items"}) {
		t.Errorf("repository access order = %v, want task permission check before item listing", accesses)
	}
}

func TestListTodoItemsUseCaseExecuteReturnsNonNilEmptyList(t *testing.T) {
	var accesses []string
	taskRepo := &listTodoItemsTaskRepositoryFake{task: dao.Task{UserID: "user-1"}, accesses: &accesses}
	itemsRepo := &listTodoItemsRepositoryFake{accesses: &accesses}
	uow := &listTodoItemsUOWFake{repos: listTodoItemsRepositoriesFake{
		tasks: taskRepo, todoItems: itemsRepo, accesses: &accesses,
	}}

	got, err := NewListTodoItemsUseCase(uow, nil).Execute(context.Background(), "user-1", "task-1")
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("Execute() = %#v, want non-nil empty list", got)
	}
}

func TestListTodoItemsUseCaseExecuteStopsWhenTodoItemReadIsDenied(t *testing.T) {
	userID := domain.UserID("user-1")
	taskID := domain.TaskID("task-1")
	var accesses []string
	taskRepo := &listTodoItemsTaskRepositoryFake{err: ErrTaskNotFound, accesses: &accesses}
	itemsRepo := &listTodoItemsRepositoryFake{items: []dao.TodoItem{{ID: "must-not-be-read"}}, accesses: &accesses}
	uow := &listTodoItemsUOWFake{repos: listTodoItemsRepositoriesFake{
		tasks: taskRepo, todoItems: itemsRepo, accesses: &accesses,
	}}

	got, err := NewListTodoItemsUseCase(uow, nil).Execute(context.Background(), userID, taskID)
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("Execute() error = %v, want %v", err, ErrTaskNotFound)
	}
	if got != nil {
		t.Errorf("Execute() = %#v, want nil result on error", got)
	}
	if itemsRepo.calls != 0 {
		t.Errorf("ListByTask() calls = %d, want 0 when todo_item/read is denied", itemsRepo.calls)
	}
}

func TestListTodoItemsUseCaseExecutePropagatesRepositoryErrors(t *testing.T) {
	userID := domain.UserID("user-1")
	taskID := domain.TaskID("task-1")
	wantErr := errors.New("database unavailable")
	t.Run("task repository", func(t *testing.T) {
		var accesses []string
		taskRepo := &listTodoItemsTaskRepositoryFake{err: wantErr, accesses: &accesses}
		itemsRepo := &listTodoItemsRepositoryFake{accesses: &accesses}
		uow := &listTodoItemsUOWFake{repos: listTodoItemsRepositoriesFake{
			tasks: taskRepo, todoItems: itemsRepo, accesses: &accesses,
		}}
		got, err := NewListTodoItemsUseCase(uow, nil).Execute(context.Background(), userID, taskID)
		if !errors.Is(err, wantErr) || got != nil || itemsRepo.calls != 0 {
			t.Errorf("Execute() = (%#v, %v), item calls %d; want nil result, original error, 0", got, err, itemsRepo.calls)
		}
	})
	t.Run("todo item repository", func(t *testing.T) {
		var accesses []string
		taskRepo := &listTodoItemsTaskRepositoryFake{task: dao.Task{UserID: string(userID)}, accesses: &accesses}
		itemsRepo := &listTodoItemsRepositoryFake{err: wantErr, accesses: &accesses}
		uow := &listTodoItemsUOWFake{repos: listTodoItemsRepositoriesFake{
			tasks: taskRepo, todoItems: itemsRepo, accesses: &accesses,
		}}
		got, err := NewListTodoItemsUseCase(uow, nil).Execute(context.Background(), userID, taskID)
		if !errors.Is(err, wantErr) || got != nil {
			t.Errorf("Execute() = (%#v, %v), want nil result and original error", got, err)
		}
	})
}

func TestListTodoItemsUseCaseExecutePropagatesUOWError(t *testing.T) {
	wantErr := errors.New("transaction failed")
	uow := &listTodoItemsUOWFake{err: wantErr}

	got, err := NewListTodoItemsUseCase(uow, nil).Execute(context.Background(), "user-1", "task-1")
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
