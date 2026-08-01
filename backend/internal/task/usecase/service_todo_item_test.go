package usecase

import (
	"context"
	"errors"
	domain "github.com/Najah7/task2todaytodo/internal/task/domain"
	"testing"
	"time"
)

func TestTodoItemServiceCreateDefaultsAppendToTailAndDedupesFrequencies(t *testing.T) {
	repo := &fakeTodoItemRepository{}
	service := NewTodoItemService(repo)
	dueDate := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

	got, err := service.Create(context.Background(), fixedTaskIDGen("todo-1"), TodoItemCreateParams{
		UserID:      "user-1",
		TaskID:      "task-1",
		Title:       "Buy milk",
		Description: "At the store",
		DueDate:     dueDate,
		Frequencies: []string{"mon", "mon", "wed"},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if got.ID != "todo-1" || got.TaskID != "task-1" || got.Position != 0 || got.IntervalWeeks != domain.OnceIntervalWeeks {
		t.Fatalf("todo item = %+v, want generated ID and one-off defaults", got)
	}
	if got.DueDate != dueDate {
		t.Fatalf("todo item = %+v, want due date from params", got)
	}
	if len(got.Frequencies) != 2 || got.Frequencies[0].String() != "mon" || got.Frequencies[1].String() != "wed" {
		t.Fatalf("frequencies = %+v, want deduped order-preserving values", got.Frequencies)
	}
	if repo.createUserID != "user-1" || !repo.appendToTail || repo.createCalls != 1 {
		t.Fatalf("repo create user=%q append=%v calls=%d, want append-to-tail create", repo.createUserID, repo.appendToTail, repo.createCalls)
	}
}

func TestTodoItemServiceCreateWithExplicitPosition(t *testing.T) {
	repo := &fakeTodoItemRepository{}
	service := NewTodoItemService(repo)
	position := 3
	intervalWeeks := 2

	got, err := service.Create(context.Background(), fixedTaskIDGen("todo-1"), TodoItemCreateParams{
		UserID:        "user-1",
		TaskID:        "task-1",
		Title:         "Buy milk",
		Position:      &position,
		IntervalWeeks: &intervalWeeks,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if got.Position != 3 || got.IntervalWeeks != 2 {
		t.Fatalf("todo item = %+v, want explicit position and interval", got)
	}
	if repo.appendToTail {
		t.Fatalf("appendToTail = true, want false for explicit position")
	}
}

func TestTodoItemServiceCreateRejectsInvalidFrequency(t *testing.T) {
	repo := &fakeTodoItemRepository{}
	service := NewTodoItemService(repo)

	_, err := service.Create(context.Background(), fixedTaskIDGen("todo-1"), TodoItemCreateParams{
		UserID:      "user-1",
		TaskID:      "task-1",
		Title:       "Buy milk",
		Frequencies: []string{"daily"},
	})
	if !errors.Is(err, domain.ErrTaskFrequencyInvalid) {
		t.Fatalf("Create() error = %v, want domain.ErrTaskFrequencyInvalid", err)
	}
	if repo.createCalls != 0 {
		t.Fatalf("createCalls = %d, want repository not called", repo.createCalls)
	}
}

func TestTodoItemServiceCreatePropagatesTaskNotFound(t *testing.T) {
	repo := &fakeTodoItemRepository{createErr: ErrTodoItemTaskNotFound}
	service := NewTodoItemService(repo)

	_, err := service.Create(context.Background(), fixedTaskIDGen("todo-1"), TodoItemCreateParams{
		UserID: "user-1",
		TaskID: "task-1",
		Title:  "Buy milk",
	})
	if !errors.Is(err, ErrTodoItemTaskNotFound) {
		t.Fatalf("Create() error = %v, want ErrTodoItemTaskNotFound", err)
	}
}

func TestTodoItemServiceOwnedOperationsScopeByUser(t *testing.T) {
	repo := &fakeTodoItemRepository{}
	service := NewTodoItemService(repo)

	if err := service.Check(context.Background(), "user-1", "task-1", "todo-1"); err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if err := service.Uncheck(context.Background(), "user-1", "task-1", "todo-1"); err != nil {
		t.Fatalf("Uncheck() error = %v", err)
	}
	if err := service.Delete(context.Background(), "user-1", "task-1", "todo-1"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if repo.checkUserID != "user-1" || repo.checkTaskID != "task-1" || repo.checkTodoItemID != "todo-1" {
		t.Fatalf("check scope = %+v, want owned todo item scope", repo)
	}
	if repo.uncheckUserID != "user-1" || repo.uncheckTaskID != "task-1" || repo.uncheckTodoItemID != "todo-1" {
		t.Fatalf("uncheck scope = %+v, want owned todo item scope", repo)
	}
	if repo.deleteUserID != "user-1" || repo.deleteTaskID != "task-1" || repo.deleteTodoItemID != "todo-1" {
		t.Fatalf("delete scope = %+v, want owned todo item scope", repo)
	}
}

func TestTodoItemServiceCheckPropagatesRepositoryError(t *testing.T) {
	repo := &fakeTodoItemRepository{checkErr: ErrTodoItemNotFound}
	service := NewTodoItemService(repo)

	err := service.Check(context.Background(), "user-1", "task-1", "todo-1")
	if !errors.Is(err, ErrTodoItemNotFound) {
		t.Fatalf("Check() error = %v, want ErrTodoItemNotFound", err)
	}
}

type fakeTodoItemRepository struct {
	TodoItemRepository

	createCalls       int
	createUserID      domain.UserID
	created           domain.TodoItem
	appendToTail      bool
	createErr         error
	listByTaskCalls   int
	listByTaskUserID  domain.UserID
	listByTaskTaskID  domain.TaskID
	listByTaskItems   domain.TodoItems
	listByTaskErr     error
	checkUserID       domain.UserID
	checkTaskID       domain.TaskID
	checkTodoItemID   domain.TodoItemID
	checkErr          error
	uncheckUserID     domain.UserID
	uncheckTaskID     domain.TaskID
	uncheckTodoItemID domain.TodoItemID
	deleteUserID      domain.UserID
	deleteTaskID      domain.TaskID
	deleteTodoItemID  domain.TodoItemID
}

func (r *fakeTodoItemRepository) CreateForOwnedTask(_ context.Context, userID domain.UserID, item domain.TodoItem, appendToTail bool) (domain.TodoItem, error) {
	r.createCalls++
	r.createUserID = userID
	r.created = item
	r.appendToTail = appendToTail
	if r.createErr != nil {
		return domain.NewZeroTodoItem(), r.createErr
	}
	return item, nil
}

func (r *fakeTodoItemRepository) ListByTask(_ context.Context, userID domain.UserID, taskID domain.TaskID) (domain.TodoItems, error) {
	r.listByTaskCalls++
	r.listByTaskUserID = userID
	r.listByTaskTaskID = taskID
	if r.listByTaskErr != nil {
		return nil, r.listByTaskErr
	}
	return r.listByTaskItems, nil
}

func (r *fakeTodoItemRepository) CheckForOwnedTask(_ context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TodoItemID) error {
	r.checkUserID = userID
	r.checkTaskID = taskID
	r.checkTodoItemID = id
	return r.checkErr
}

func (r *fakeTodoItemRepository) UncheckForOwnedTask(_ context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TodoItemID) error {
	r.uncheckUserID = userID
	r.uncheckTaskID = taskID
	r.uncheckTodoItemID = id
	return nil
}

func (r *fakeTodoItemRepository) DeleteForOwnedTask(_ context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TodoItemID) error {
	r.deleteUserID = userID
	r.deleteTaskID = taskID
	r.deleteTodoItemID = id
	return nil
}
