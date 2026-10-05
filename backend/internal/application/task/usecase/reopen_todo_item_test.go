package usecase

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

type reopenTodoItemUOWFake struct {
	repos Repositories
	err   error
	calls int
}

func (uow *reopenTodoItemUOWFake) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	uow.calls++
	if uow.err != nil {
		return uow.err
	}
	return fn(ctx, uow.repos)
}

type reopenTodoItemRepositoriesFake struct {
	taskProgressTestRepositories
	repo TodoItemRepository
}

func (repos reopenTodoItemRepositoriesFake) TodoItems() TodoItemRepository {
	return repos.repo
}

type reopenTodoItemState struct {
	completed     bool
	intervalWeeks int
	seriesID      domain.TodoItemID
}

type reopenTodoItemRepositoryFake struct {
	TodoItemRepository
	items  map[string]reopenTodoItemState
	userID domain.UserID
	taskID domain.TaskID
	itemID domain.TodoItemID
	calls  int
	err    error
}

func (repo *reopenTodoItemRepositoryFake) UncheckForOwnedTask(_ context.Context, userID domain.UserID, taskID domain.TaskID, itemID domain.TodoItemID) error {
	repo.calls++
	repo.userID, repo.taskID, repo.itemID = userID, taskID, itemID
	if repo.err != nil {
		return repo.err
	}
	key := reopenTodoItemKey(userID, taskID, itemID)
	item, exists := repo.items[key]
	if !exists {
		return ErrTodoItemNotFound
	}
	item.completed = false
	repo.items[key] = item
	return nil
}

func reopenTodoItemKey(userID domain.UserID, taskID domain.TaskID, itemID domain.TodoItemID) string {
	return fmt.Sprintf("%s/%s/%s", userID, taskID, itemID)
}

func newReopenTodoItemFixture(items map[string]reopenTodoItemState) (*reopenTodoItemUOWFake, *reopenTodoItemRepositoryFake) {
	repo := &reopenTodoItemRepositoryFake{items: items}
	uow := &reopenTodoItemUOWFake{repos: reopenTodoItemRepositoriesFake{repo: repo}}
	return uow, repo
}

func TestReopenTodoItemUseCaseReopensOneTimeItem(t *testing.T) {
	userID, taskID, itemID := domain.UserID("user-1"), domain.TaskID("task-1"), domain.TodoItemID("item-1")
	key := reopenTodoItemKey(userID, taskID, itemID)
	uow, repo := newReopenTodoItemFixture(map[string]reopenTodoItemState{
		key: {completed: true},
	})

	if err := NewReopenTodoItemUseCase(uow, nil).Execute(context.Background(), userID, taskID, itemID); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if repo.items[key].completed {
		t.Error("item is still completed")
	}
	if uow.calls != 1 || repo.calls != 1 {
		t.Errorf("calls = UOW:%d repository:%d, want 1 each", uow.calls, repo.calls)
	}
	if repo.userID != userID || repo.taskID != taskID || repo.itemID != itemID {
		t.Errorf("repository arguments = (%q, %q, %q), want (%q, %q, %q)", repo.userID, repo.taskID, repo.itemID, userID, taskID, itemID)
	}
}

func TestReopenTodoItemUseCaseIsIdempotentAndPreservesRecurrence(t *testing.T) {
	userID, taskID, sourceID := domain.UserID("user-1"), domain.TaskID("task-1"), domain.TodoItemID("series-root")
	key := reopenTodoItemKey(userID, taskID, sourceID)
	uow, repo := newReopenTodoItemFixture(map[string]reopenTodoItemState{
		key: {completed: true, intervalWeeks: 2, seriesID: sourceID},
	})
	useCase := NewReopenTodoItemUseCase(uow, nil)

	for range 2 {
		if err := useCase.Execute(context.Background(), userID, taskID, sourceID); err != nil {
			t.Fatalf("Execute() error = %v, want nil", err)
		}
	}
	item := repo.items[key]
	if item.completed {
		t.Error("source occurrence is still completed")
	}
	if item.intervalWeeks != 2 || item.seriesID != sourceID {
		t.Errorf("recurrence changed after reopen: interval=%d series=%q", item.intervalWeeks, item.seriesID)
	}
	if uow.calls != 2 || repo.calls != 2 {
		t.Errorf("repeated reopen calls = UOW:%d repository:%d, want 2 each", uow.calls, repo.calls)
	}
}

func TestReopenTodoItemUseCaseLeavesOtherOccurrenceUnchanged(t *testing.T) {
	userID, taskID := domain.UserID("user-1"), domain.TaskID("task-1")
	itemID, otherOccurrenceID := domain.TodoItemID("occurrence-today"), domain.TodoItemID("occurrence-next-week")
	key := reopenTodoItemKey(userID, taskID, itemID)
	otherKey := reopenTodoItemKey(userID, taskID, otherOccurrenceID)
	uow, repo := newReopenTodoItemFixture(map[string]reopenTodoItemState{
		key:      {completed: true, intervalWeeks: 1, seriesID: "series-root"},
		otherKey: {completed: true, intervalWeeks: 1, seriesID: "series-root"},
	})

	if err := NewReopenTodoItemUseCase(uow, nil).Execute(context.Background(), userID, taskID, itemID); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if repo.items[key].completed {
		t.Error("requested occurrence is still completed")
	}
	if !repo.items[otherKey].completed {
		t.Error("other occurrence was reopened as a side effect")
	}
}

func TestReopenTodoItemUseCaseReturnsNotFoundForAnotherUsersItem(t *testing.T) {
	ownerID, taskID, itemID := domain.UserID("owner"), domain.TaskID("task-1"), domain.TodoItemID("item-1")
	ownerKey := reopenTodoItemKey(ownerID, taskID, itemID)
	uow, repo := newReopenTodoItemFixture(map[string]reopenTodoItemState{
		ownerKey: {completed: true},
	})

	err := NewReopenTodoItemUseCase(uow, nil).Execute(context.Background(), "other-user", taskID, itemID)
	if !errors.Is(err, ErrTodoItemNotFound) {
		t.Errorf("Execute() error = %v, want %v", err, ErrTodoItemNotFound)
	}
	if !repo.items[ownerKey].completed {
		t.Error("item owned by another user was reopened")
	}
	if repo.calls != 1 || repo.userID != "other-user" {
		t.Errorf("repository calls = %d, user = %q; want 1 call for requesting user", repo.calls, repo.userID)
	}
}

func TestReopenTodoItemUseCasePropagatesRepositoryAndUOWErrors(t *testing.T) {
	userID, taskID, itemID := domain.UserID("user-1"), domain.TaskID("task-1"), domain.TodoItemID("item-1")
	uowErr := errors.New("transaction failed")
	uow := &reopenTodoItemUOWFake{err: uowErr}
	if err := NewReopenTodoItemUseCase(uow, nil).Execute(context.Background(), userID, taskID, itemID); !errors.Is(err, uowErr) {
		t.Errorf("UOW error = %v, want %v", err, uowErr)
	}

	uow, repo := newReopenTodoItemFixture(nil)
	repo.err = errors.New("database unavailable")
	if err := NewReopenTodoItemUseCase(uow, nil).Execute(context.Background(), userID, taskID, itemID); !errors.Is(err, repo.err) {
		t.Errorf("repository error = %v, want %v", err, repo.err)
	}
}
