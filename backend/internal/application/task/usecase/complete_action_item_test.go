package usecase

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

type completeActionItemUOWFake struct {
	repos Repositories
	err   error
	calls int
}

func (uow *completeActionItemUOWFake) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	uow.calls++
	if uow.err != nil {
		return uow.err
	}
	return fn(ctx, uow.repos)
}

type completeActionItemRepositoriesFake struct {
	taskProgressTestRepositories
	repo ActionItemRepository
}

func (repos completeActionItemRepositoriesFake) ActionItems() ActionItemRepository {
	return repos.repo
}

type completeActionItemState struct {
	completed     bool
	intervalWeeks int
	frequencies   []dao.TaskFrequency
}

type completeActionItemRepositoryFake struct {
	ActionItemRepository
	items  map[string]completeActionItemState
	userID domain.UserID
	taskID domain.TaskID
	itemID domain.ActionItemID
	calls  int
	err    error
}

func (*completeActionItemRepositoryFake) ReadTaskListProjection(_ context.Context, _ domain.UserID, taskIDs []string) (dao.TaskListProjectionSources, error) {
	items := make(map[string][]dao.ActionItem, len(taskIDs))
	skipped := make(map[string]map[string]map[string]bool, len(taskIDs))
	for _, id := range taskIDs {
		items[id] = nil
		skipped[id] = nil
	}
	return dao.TaskListProjectionSources{ActionItemsByTask: items, SkippedByTask: skipped}, nil
}

func (repo *completeActionItemRepositoryFake) CheckForOwnedTask(_ context.Context, userID domain.UserID, taskID domain.TaskID, itemID domain.ActionItemID) error {
	repo.calls++
	repo.userID, repo.taskID, repo.itemID = userID, taskID, itemID
	if repo.err != nil {
		return repo.err
	}
	key := completeActionItemKey(userID, taskID, itemID)
	item, exists := repo.items[key]
	if !exists {
		return ErrActionItemNotFound
	}
	item.completed = true
	repo.items[key] = item
	return nil
}

func completeActionItemKey(userID domain.UserID, taskID domain.TaskID, itemID domain.ActionItemID) string {
	return fmt.Sprintf("%s/%s/%s", userID, taskID, itemID)
}

func newCompleteActionItemFixture(items map[string]completeActionItemState) (*completeActionItemUOWFake, *completeActionItemRepositoryFake) {
	repo := &completeActionItemRepositoryFake{items: items}
	uow := &completeActionItemUOWFake{repos: completeActionItemRepositoriesFake{repo: repo}}
	return uow, repo
}

func TestCompleteActionItemUseCaseCompletesOneTimeItem(t *testing.T) {
	userID, taskID, itemID := domain.UserID("user-1"), domain.TaskID("task-1"), domain.ActionItemID("item-1")
	key := completeActionItemKey(userID, taskID, itemID)
	uow, repo := newCompleteActionItemFixture(map[string]completeActionItemState{
		key: {intervalWeeks: 0},
	})

	if err := NewCompleteActionItemUseCase(uow, nil).Execute(context.Background(), userID, taskID, itemID); err != nil {
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

func TestCompleteActionItemUseCaseCompletesRecurringSourceAndIsIdempotent(t *testing.T) {
	userID, taskID, sourceID := domain.UserID("user-1"), domain.TaskID("task-1"), domain.ActionItemID("series-root")
	key := completeActionItemKey(userID, taskID, sourceID)
	frequencies := []dao.TaskFrequency{{Value: "monday", Label: "Monday"}}
	uow, repo := newCompleteActionItemFixture(map[string]completeActionItemState{
		key: {intervalWeeks: 1, frequencies: frequencies},
	})
	useCase := NewCompleteActionItemUseCase(uow, nil)

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

func TestCompleteActionItemUseCaseLeavesFutureOccurrenceIndependent(t *testing.T) {
	userID, taskID := domain.UserID("user-1"), domain.TaskID("task-1")
	completedID, futureID := domain.ActionItemID("occurrence-today"), domain.ActionItemID("occurrence-future")
	completedKey := completeActionItemKey(userID, taskID, completedID)
	futureKey := completeActionItemKey(userID, taskID, futureID)
	uow, repo := newCompleteActionItemFixture(map[string]completeActionItemState{
		completedKey: {intervalWeeks: 1},
		futureKey:    {intervalWeeks: 1},
	})

	if err := NewCompleteActionItemUseCase(uow, nil).Execute(context.Background(), userID, taskID, completedID); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if !repo.items[completedKey].completed {
		t.Error("requested occurrence is not completed")
	}
	if repo.items[futureKey].completed {
		t.Error("future occurrence was completed as a side effect")
	}
}

func TestCompleteActionItemUseCaseReturnsNotFoundForAnotherUsersItem(t *testing.T) {
	ownerID, taskID, itemID := domain.UserID("owner"), domain.TaskID("task-1"), domain.ActionItemID("item-1")
	ownerKey := completeActionItemKey(ownerID, taskID, itemID)
	uow, repo := newCompleteActionItemFixture(map[string]completeActionItemState{
		ownerKey: {},
	})

	err := NewCompleteActionItemUseCase(uow, nil).Execute(context.Background(), "other-user", taskID, itemID)
	if !errors.Is(err, ErrActionItemNotFound) {
		t.Errorf("Execute() error = %v, want %v", err, ErrActionItemNotFound)
	}
	if repo.items[ownerKey].completed {
		t.Error("item owned by another user was completed")
	}
	if repo.calls != 1 || repo.userID != "other-user" {
		t.Errorf("repository calls = %d, user = %q; want 1 call for requesting user", repo.calls, repo.userID)
	}
}

func TestCompleteActionItemUseCasePropagatesRepositoryAndUOWErrors(t *testing.T) {
	userID, taskID, itemID := domain.UserID("user-1"), domain.TaskID("task-1"), domain.ActionItemID("item-1")
	uowErr := errors.New("transaction failed")
	uow := &completeActionItemUOWFake{err: uowErr}
	if err := NewCompleteActionItemUseCase(uow, nil).Execute(context.Background(), userID, taskID, itemID); !errors.Is(err, uowErr) {
		t.Errorf("UOW error = %v, want %v", err, uowErr)
	}

	repoErr := errors.New("database unavailable")
	uow, repo := newCompleteActionItemFixture(nil)
	repo.err = repoErr
	if err := NewCompleteActionItemUseCase(uow, nil).Execute(context.Background(), userID, taskID, itemID); !errors.Is(err, repoErr) {
		t.Errorf("repository error = %v, want %v", err, repoErr)
	}
}
