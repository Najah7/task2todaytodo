package usecase

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

type completeTodoItemUOWFake struct {
	repos Repositories
	err   error
	calls int
}

func (uow *completeTodoItemUOWFake) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	uow.calls++
	if uow.err != nil {
		return uow.err
	}
	return fn(ctx, uow.repos)
}

type completeTodoItemRepositoriesFake struct {
	taskProgressTestRepositories
	repo TodoItemRepository
}

func (repos completeTodoItemRepositoriesFake) TodoItems() TodoItemRepository {
	return repos.repo
}

type completeTodoItemState struct {
	completed     bool
	intervalWeeks int
	frequencies   []dao.TaskFrequency
}

type completeTodoItemRepositoryFake struct {
	TodoItemRepository
	items  map[string]completeTodoItemState
	userID domain.UserID
	taskID domain.TaskID
	itemID domain.TodoItemID
	calls  int
	err    error
}

func (repo *completeTodoItemRepositoryFake) CheckForOwnedTask(_ context.Context, userID domain.UserID, taskID domain.TaskID, itemID domain.TodoItemID) error {
	repo.calls++
	repo.userID, repo.taskID, repo.itemID = userID, taskID, itemID
	if repo.err != nil {
		return repo.err
	}
	key := completeTodoItemKey(userID, taskID, itemID)
	item, exists := repo.items[key]
	if !exists {
		return ErrTodoItemNotFound
	}
	item.completed = true
	repo.items[key] = item
	return nil
}

func completeTodoItemKey(userID domain.UserID, taskID domain.TaskID, itemID domain.TodoItemID) string {
	return fmt.Sprintf("%s/%s/%s", userID, taskID, itemID)
}

func newCompleteTodoItemFixture(items map[string]completeTodoItemState) (*completeTodoItemUOWFake, *completeTodoItemRepositoryFake) {
	repo := &completeTodoItemRepositoryFake{items: items}
	uow := &completeTodoItemUOWFake{repos: completeTodoItemRepositoriesFake{repo: repo}}
	return uow, repo
}

func TestCompleteTodoItemUseCaseCompletesOneTimeItem(t *testing.T) {
	userID, taskID, itemID := domain.UserID("user-1"), domain.TaskID("task-1"), domain.TodoItemID("item-1")
	key := completeTodoItemKey(userID, taskID, itemID)
	uow, repo := newCompleteTodoItemFixture(map[string]completeTodoItemState{
		key: {intervalWeeks: 0},
	})

	if err := NewCompleteTodoItemUseCase(uow, nil).Execute(context.Background(), userID, taskID, itemID); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if got := repo.items[key].completed; !got {
		t.Errorf("item completed = %t, want true", got)
	}
	if uow.calls != 1 || repo.calls != 1 {
		t.Errorf("calls = UOW:%d repository:%d, want 1 each", uow.calls, repo.calls)
	}
	if repo.userID != userID || repo.taskID != taskID || repo.itemID != itemID {
		t.Errorf("repository arguments = (%q, %q, %q), want (%q, %q, %q)", repo.userID, repo.taskID, repo.itemID, userID, taskID, itemID)
	}
}

func TestCompleteTodoItemUseCaseCompletesRecurringSourceAndIsIdempotent(t *testing.T) {
	userID, taskID, sourceID := domain.UserID("user-1"), domain.TaskID("task-1"), domain.TodoItemID("series-root")
	key := completeTodoItemKey(userID, taskID, sourceID)
	frequencies := []dao.TaskFrequency{{Value: "monday", Label: "Monday"}}
	uow, repo := newCompleteTodoItemFixture(map[string]completeTodoItemState{
		key: {intervalWeeks: 1, frequencies: frequencies},
	})
	useCase := NewCompleteTodoItemUseCase(uow, nil)

	for range 2 {
		if err := useCase.Execute(context.Background(), userID, taskID, sourceID); err != nil {
			t.Fatalf("Execute() error = %v, want nil", err)
		}
	}
	item := repo.items[key]
	if !item.completed {
		t.Error("recurring source is not completed")
	}
	if item.intervalWeeks != 1 || len(item.frequencies) != 1 || item.frequencies[0] != frequencies[0] {
		t.Errorf("recurrence changed after completion: interval=%d frequencies=%v", item.intervalWeeks, item.frequencies)
	}
	if uow.calls != 2 || repo.calls != 2 {
		t.Errorf("repeated completion calls = UOW:%d repository:%d, want 2 each", uow.calls, repo.calls)
	}
}

func TestCompleteTodoItemUseCaseLeavesFutureOccurrenceIndependent(t *testing.T) {
	userID, taskID := domain.UserID("user-1"), domain.TaskID("task-1")
	completedID, futureID := domain.TodoItemID("occurrence-today"), domain.TodoItemID("occurrence-future")
	completedKey := completeTodoItemKey(userID, taskID, completedID)
	futureKey := completeTodoItemKey(userID, taskID, futureID)
	uow, repo := newCompleteTodoItemFixture(map[string]completeTodoItemState{
		completedKey: {intervalWeeks: 1},
		futureKey:    {intervalWeeks: 1},
	})

	if err := NewCompleteTodoItemUseCase(uow, nil).Execute(context.Background(), userID, taskID, completedID); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if !repo.items[completedKey].completed {
		t.Error("requested occurrence is not completed")
	}
	if repo.items[futureKey].completed {
		t.Error("future occurrence was completed as a side effect")
	}
}

func TestCompleteTodoItemUseCaseReturnsNotFoundForAnotherUsersItem(t *testing.T) {
	ownerID, taskID, itemID := domain.UserID("owner"), domain.TaskID("task-1"), domain.TodoItemID("item-1")
	ownerKey := completeTodoItemKey(ownerID, taskID, itemID)
	uow, repo := newCompleteTodoItemFixture(map[string]completeTodoItemState{
		ownerKey: {},
	})

	err := NewCompleteTodoItemUseCase(uow, nil).Execute(context.Background(), "other-user", taskID, itemID)
	if !errors.Is(err, ErrTodoItemNotFound) {
		t.Errorf("Execute() error = %v, want %v", err, ErrTodoItemNotFound)
	}
	if repo.items[ownerKey].completed {
		t.Error("item owned by another user was completed")
	}
	if repo.calls != 1 || repo.userID != "other-user" {
		t.Errorf("repository calls = %d, user = %q; want 1 call for requesting user", repo.calls, repo.userID)
	}
}

func TestCompleteTodoItemUseCasePropagatesRepositoryAndUOWErrors(t *testing.T) {
	userID, taskID, itemID := domain.UserID("user-1"), domain.TaskID("task-1"), domain.TodoItemID("item-1")
	uowErr := errors.New("transaction failed")
	uow := &completeTodoItemUOWFake{err: uowErr}
	if err := NewCompleteTodoItemUseCase(uow, nil).Execute(context.Background(), userID, taskID, itemID); !errors.Is(err, uowErr) {
		t.Errorf("UOW error = %v, want %v", err, uowErr)
	}

	repoErr := errors.New("database unavailable")
	uow, repo := newCompleteTodoItemFixture(nil)
	repo.err = repoErr
	if err := NewCompleteTodoItemUseCase(uow, nil).Execute(context.Background(), userID, taskID, itemID); !errors.Is(err, repoErr) {
		t.Errorf("repository error = %v, want %v", err, repoErr)
	}
}
