package usecase

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

type reopenActionItemUOWFake struct {
	repos Repositories
	err   error
	calls int
}

func (uow *reopenActionItemUOWFake) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	uow.calls++
	if uow.err != nil {
		return uow.err
	}
	return fn(ctx, uow.repos)
}

type reopenActionItemRepositoriesFake struct {
	taskProgressTestRepositories
	repo ActionItemRepository
}

func (repos reopenActionItemRepositoriesFake) ActionItems() ActionItemRepository {
	return repos.repo
}

type reopenActionItemState struct {
	completed     bool
	intervalWeeks int
	seriesID      domain.ActionItemID
}

type reopenActionItemRepositoryFake struct {
	ActionItemRepository
	items  map[string]reopenActionItemState
	userID domain.UserID
	taskID domain.TaskID
	itemID domain.ActionItemID
	calls  int
	err    error
}

func (*reopenActionItemRepositoryFake) ReadTaskListProjection(_ context.Context, _ domain.UserID, taskIDs []string) (dao.TaskListProjectionSources, error) {
	items := make(map[string][]dao.ActionItem, len(taskIDs))
	skipped := make(map[string]map[string]map[string]bool, len(taskIDs))
	for _, id := range taskIDs {
		items[id] = nil
		skipped[id] = nil
	}
	return dao.TaskListProjectionSources{ActionItemsByTask: items, SkippedByTask: skipped}, nil
}

func (repo *reopenActionItemRepositoryFake) UncheckForOwnedTask(_ context.Context, userID domain.UserID, taskID domain.TaskID, itemID domain.ActionItemID) error {
	repo.calls++
	repo.userID, repo.taskID, repo.itemID = userID, taskID, itemID
	if repo.err != nil {
		return repo.err
	}
	key := reopenActionItemKey(userID, taskID, itemID)
	item, exists := repo.items[key]
	if !exists {
		return ErrActionItemNotFound
	}
	item.completed = false
	repo.items[key] = item
	return nil
}

func reopenActionItemKey(userID domain.UserID, taskID domain.TaskID, itemID domain.ActionItemID) string {
	return fmt.Sprintf("%s/%s/%s", userID, taskID, itemID)
}

func newReopenActionItemFixture(items map[string]reopenActionItemState) (*reopenActionItemUOWFake, *reopenActionItemRepositoryFake) {
	repo := &reopenActionItemRepositoryFake{items: items}
	uow := &reopenActionItemUOWFake{repos: reopenActionItemRepositoriesFake{repo: repo}}
	return uow, repo
}

func TestReopenActionItemUseCaseReopensOneTimeItem(t *testing.T) {
	userID, taskID, itemID := domain.UserID("user-1"), domain.TaskID("task-1"), domain.ActionItemID("item-1")
	key := reopenActionItemKey(userID, taskID, itemID)
	uow, repo := newReopenActionItemFixture(map[string]reopenActionItemState{
		key: {completed: true},
	})

	if err := NewReopenActionItemUseCase(uow, nil).Execute(context.Background(), userID, taskID, itemID); err != nil {
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

func TestReopenActionItemUseCaseIsIdempotentAndPreservesRecurrence(t *testing.T) {
	userID, taskID, sourceID := domain.UserID("user-1"), domain.TaskID("task-1"), domain.ActionItemID("series-root")
	key := reopenActionItemKey(userID, taskID, sourceID)
	uow, repo := newReopenActionItemFixture(map[string]reopenActionItemState{
		key: {completed: true, intervalWeeks: 2, seriesID: sourceID},
	})
	useCase := NewReopenActionItemUseCase(uow, nil)

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

func TestReopenActionItemUseCaseLeavesOtherOccurrenceUnchanged(t *testing.T) {
	userID, taskID := domain.UserID("user-1"), domain.TaskID("task-1")
	itemID, otherOccurrenceID := domain.ActionItemID("occurrence-today"), domain.ActionItemID("occurrence-next-week")
	key := reopenActionItemKey(userID, taskID, itemID)
	otherKey := reopenActionItemKey(userID, taskID, otherOccurrenceID)
	uow, repo := newReopenActionItemFixture(map[string]reopenActionItemState{
		key:      {completed: true, intervalWeeks: 1, seriesID: "series-root"},
		otherKey: {completed: true, intervalWeeks: 1, seriesID: "series-root"},
	})

	if err := NewReopenActionItemUseCase(uow, nil).Execute(context.Background(), userID, taskID, itemID); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if repo.items[key].completed {
		t.Error("requested occurrence is still completed")
	}
	if !repo.items[otherKey].completed {
		t.Error("other occurrence was reopened as a side effect")
	}
}

func TestReopenActionItemUseCaseReturnsNotFoundForAnotherUsersItem(t *testing.T) {
	ownerID, taskID, itemID := domain.UserID("owner"), domain.TaskID("task-1"), domain.ActionItemID("item-1")
	ownerKey := reopenActionItemKey(ownerID, taskID, itemID)
	uow, repo := newReopenActionItemFixture(map[string]reopenActionItemState{
		ownerKey: {completed: true},
	})

	err := NewReopenActionItemUseCase(uow, nil).Execute(context.Background(), "other-user", taskID, itemID)
	if !errors.Is(err, ErrActionItemNotFound) {
		t.Errorf("Execute() error = %v, want %v", err, ErrActionItemNotFound)
	}
	if !repo.items[ownerKey].completed {
		t.Error("item owned by another user was reopened")
	}
	if repo.calls != 1 || repo.userID != "other-user" {
		t.Errorf("repository calls = %d, user = %q; want 1 call for requesting user", repo.calls, repo.userID)
	}
}

func TestReopenActionItemUseCasePropagatesRepositoryAndUOWErrors(t *testing.T) {
	userID, taskID, itemID := domain.UserID("user-1"), domain.TaskID("task-1"), domain.ActionItemID("item-1")
	uowErr := errors.New("transaction failed")
	uow := &reopenActionItemUOWFake{err: uowErr}
	if err := NewReopenActionItemUseCase(uow, nil).Execute(context.Background(), userID, taskID, itemID); !errors.Is(err, uowErr) {
		t.Errorf("UOW error = %v, want %v", err, uowErr)
	}

	uow, repo := newReopenActionItemFixture(nil)
	repo.err = errors.New("database unavailable")
	if err := NewReopenActionItemUseCase(uow, nil).Execute(context.Background(), userID, taskID, itemID); !errors.Is(err, repo.err) {
		t.Errorf("repository error = %v, want %v", err, repo.err)
	}
}
