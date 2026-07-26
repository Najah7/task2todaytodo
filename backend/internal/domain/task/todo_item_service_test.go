package task

import (
	"context"
	"errors"
	"testing"
)

func TestTodoItemServiceCreateDefaultsAppendToTailAndDedupesFrequencies(t *testing.T) {
	repo := &fakeTodoItemRepository{}
	service := NewTodoItemService(repo)

	got, err := service.Create(context.Background(), fixedTaskIDGen("todo-1"), TodoItemCreateParams{
		UserID:      "user-1",
		TaskID:      "task-1",
		Title:       "Buy milk",
		Description: "At the store",
		Frequencies: []string{"mon", "mon", "wed"},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if got.ID != "todo-1" || got.TaskID != "task-1" || got.Position != 0 || got.IntervalWeeks != OnceIntervalWeeks {
		t.Fatalf("todo item = %+v, want generated ID and one-off defaults", got)
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
	if !errors.Is(err, ErrTaskFrequencyInvalid) {
		t.Fatalf("Create() error = %v, want ErrTaskFrequencyInvalid", err)
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
	createUserID      UserID
	created           TodoItem
	appendToTail      bool
	createErr         error
	checkUserID       UserID
	checkTaskID       TaskID
	checkTodoItemID   TodoItemID
	checkErr          error
	uncheckUserID     UserID
	uncheckTaskID     TaskID
	uncheckTodoItemID TodoItemID
	deleteUserID      UserID
	deleteTaskID      TaskID
	deleteTodoItemID  TodoItemID
}

func (r *fakeTodoItemRepository) CreateForOwnedTask(_ context.Context, userID UserID, item TodoItem, appendToTail bool) (TodoItem, error) {
	r.createCalls++
	r.createUserID = userID
	r.created = item
	r.appendToTail = appendToTail
	if r.createErr != nil {
		return NewZeroTodoItem(), r.createErr
	}
	return item, nil
}

func (r *fakeTodoItemRepository) CheckForOwnedTask(_ context.Context, userID UserID, taskID TaskID, id TodoItemID) error {
	r.checkUserID = userID
	r.checkTaskID = taskID
	r.checkTodoItemID = id
	return r.checkErr
}

func (r *fakeTodoItemRepository) UncheckForOwnedTask(_ context.Context, userID UserID, taskID TaskID, id TodoItemID) error {
	r.uncheckUserID = userID
	r.uncheckTaskID = taskID
	r.uncheckTodoItemID = id
	return nil
}

func (r *fakeTodoItemRepository) DeleteForOwnedTask(_ context.Context, userID UserID, taskID TaskID, id TodoItemID) error {
	r.deleteUserID = userID
	r.deleteTaskID = taskID
	r.deleteTodoItemID = id
	return nil
}
