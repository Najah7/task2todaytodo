package usecase

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

type deleteActionItemUOWFake struct {
	repos       Repositories
	err         error
	calls       int
	callbackErr error
}

func (uow *deleteActionItemUOWFake) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	uow.calls++
	if uow.err != nil {
		return uow.err
	}
	uow.callbackErr = fn(ctx, uow.repos)
	return uow.callbackErr
}

type deleteActionItemRepositoriesFake struct {
	taskProgressTestRepositories
	repo ActionItemRepository

	accesses *[]string
}

func (repos deleteActionItemRepositoriesFake) ActionItems() ActionItemRepository {
	*repos.accesses = append(*repos.accesses, "action-items")
	return repos.repo
}

type deleteActionItemRepositoryFake struct {
	ActionItemRepository
	item dao.ActionItem

	getErr           error
	tombstoneErr     error
	deleteFutureErr  error
	userID           domain.UserID
	taskID           domain.TaskID
	itemID           domain.ActionItemID
	seriesID         domain.ActionItemID
	fromDate         time.Time
	getCalls         int
	tombstoneCalls   int
	deleteFutureCall int
	accesses         *[]string
}

func (*deleteActionItemRepositoryFake) ReadTaskListProjection(_ context.Context, _ domain.UserID, taskIDs []string) (dao.TaskListProjectionSources, error) {
	items := make(map[string][]dao.ActionItem, len(taskIDs))
	skipped := make(map[string]map[string]map[string]bool, len(taskIDs))
	for _, id := range taskIDs {
		items[id] = nil
		skipped[id] = nil
	}
	return dao.TaskListProjectionSources{ActionItemsByTask: items, SkippedByTask: skipped}, nil
}

func (repo *deleteActionItemRepositoryFake) GetForOwnedTask(_ context.Context, userID domain.UserID, taskID domain.TaskID, itemID domain.ActionItemID) (dao.ActionItem, error) {
	repo.getCalls++
	repo.userID, repo.taskID, repo.itemID = userID, taskID, itemID
	*repo.accesses = append(*repo.accesses, "get-owned-item")
	if repo.getErr != nil {
		return dao.ActionItem{}, repo.getErr
	}
	return repo.item, nil
}

func (repo *deleteActionItemRepositoryFake) TombstoneForOwnedTask(_ context.Context, userID domain.UserID, taskID domain.TaskID, itemID domain.ActionItemID) error {
	repo.tombstoneCalls++
	repo.userID, repo.taskID, repo.itemID = userID, taskID, itemID
	*repo.accesses = append(*repo.accesses, "tombstone-item")
	return repo.tombstoneErr
}

func (repo *deleteActionItemRepositoryFake) DeleteUneditedFutureBySeries(_ context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.ActionItemID, fromDate time.Time) (int64, error) {
	repo.deleteFutureCall++
	repo.userID, repo.taskID, repo.seriesID, repo.fromDate = userID, taskID, seriesID, fromDate
	*repo.accesses = append(*repo.accesses, "delete-unedited-future")
	return 0, repo.deleteFutureErr
}

func newDeleteActionItemFixture(item dao.ActionItem) (*deleteActionItemUOWFake, *deleteActionItemRepositoryFake, *[]string) {
	var accesses []string
	repo := &deleteActionItemRepositoryFake{item: item, accesses: &accesses}
	uow := &deleteActionItemUOWFake{repos: deleteActionItemRepositoriesFake{repo: repo, accesses: &accesses}}
	return uow, repo, &accesses
}

func TestDeleteActionItemUseCaseTombstonesRecurringRootAndKeepsHistory(t *testing.T) {
	userID, taskID, itemID := domain.UserID("user-1"), domain.TaskID("task-1"), domain.ActionItemID("series-root")
	uow, repo, accesses := newDeleteActionItemFixture(dao.ActionItem{
		ID:            string(itemID),
		TaskID:        string(taskID),
		IntervalWeeks: 1,
		SeriesID:      string(itemID),
		Timezone:      "Asia/Tokyo",
	})

	if err := NewDeleteActionItemUseCase(uow, nil).Execute(context.Background(), userID, taskID, itemID); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if uow.calls != 1 || repo.getCalls != 1 || repo.tombstoneCalls != 1 || repo.deleteFutureCall != 0 {
		t.Errorf("calls = UOW:%d get:%d tombstone:%d delete future:%d", uow.calls, repo.getCalls, repo.tombstoneCalls, repo.deleteFutureCall)
	}
	if repo.userID != userID || repo.taskID != taskID || repo.itemID != itemID {
		t.Errorf("repository scope = user:%q task:%q item:%q, want %q/%q/%q", repo.userID, repo.taskID, repo.itemID, userID, taskID, itemID)
	}
	if !reflect.DeepEqual(*accesses, []string{"action-items", "action-items", "get-owned-item", "tombstone-item", "action-items"}) {
		t.Errorf("repository access order = %v, want load and tombstone root", *accesses)
	}
}

func TestDeleteActionItemUseCaseTombstonesSingleOccurrenceWithoutDeletingSeries(t *testing.T) {
	userID, taskID, itemID := domain.UserID("user-1"), domain.TaskID("task-1"), domain.ActionItemID("occurrence-2026-10-04")
	uow, repo, accesses := newDeleteActionItemFixture(dao.ActionItem{
		ID:       string(itemID),
		TaskID:   string(taskID),
		SeriesID: "series-root",
		Timezone: "Asia/Tokyo",
	})

	if err := NewDeleteActionItemUseCase(uow, nil).Execute(context.Background(), userID, taskID, itemID); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if repo.getCalls != 1 || repo.tombstoneCalls != 1 || repo.deleteFutureCall != 0 {
		t.Errorf("calls = get:%d tombstone:%d delete future:%d, want 1, 1, 0", repo.getCalls, repo.tombstoneCalls, repo.deleteFutureCall)
	}
	if repo.userID != userID || repo.taskID != taskID || repo.itemID != itemID {
		t.Errorf("tombstone scope = (%q, %q, %q), want (%q, %q, %q)", repo.userID, repo.taskID, repo.itemID, userID, taskID, itemID)
	}
	if !reflect.DeepEqual(*accesses, []string{"action-items", "action-items", "get-owned-item", "tombstone-item", "action-items"}) {
		t.Errorf("repository access order = %v, want only load and tombstone this occurrence", *accesses)
	}
}

func TestDeleteActionItemUseCaseRejectsCompletedOneOff(t *testing.T) {
	uow, repo, _ := newDeleteActionItemFixture(dao.ActionItem{ID: "one-off", SeriesID: "one-off", Completed: true, RepeatState: repeatStateOneOff, Timezone: "UTC"})
	err := NewDeleteActionItemUseCase(uow, nil).Execute(context.Background(), "user-1", "task-1", "one-off")
	if !errors.Is(err, ErrOccurrenceCompleted) || repo.tombstoneCalls != 0 {
		t.Fatalf("Execute() error=%v tombstones=%d; want completed conflict and no mutation", err, repo.tombstoneCalls)
	}
}

func TestDeleteActionItemUseCaseReturnsNotFoundForMissingOrUnownedItem(t *testing.T) {
	userID, taskID, itemID := domain.UserID("other-user"), domain.TaskID("task-1"), domain.ActionItemID("item-1")
	uow, repo, _ := newDeleteActionItemFixture(dao.ActionItem{ID: "item-1", SeriesID: "item-1", Timezone: "UTC"})
	repo.getErr = ErrActionItemNotFound

	err := NewDeleteActionItemUseCase(uow, nil).Execute(context.Background(), userID, taskID, itemID)
	if !errors.Is(err, ErrActionItemNotFound) {
		t.Errorf("Execute() error = %v, want %v", err, ErrActionItemNotFound)
	}
	if repo.getCalls != 1 || repo.tombstoneCalls != 0 || repo.deleteFutureCall != 0 {
		t.Errorf("calls = get:%d tombstone:%d delete future:%d, want 1, 0, 0", repo.getCalls, repo.tombstoneCalls, repo.deleteFutureCall)
	}
	if repo.userID != userID {
		t.Errorf("GetForOwnedTask() user ID = %q, want caller %q", repo.userID, userID)
	}
}

func TestDeleteActionItemUseCaseStopsWhenRootTombstoneFails(t *testing.T) {
	wantErr := errors.New("database unavailable")
	uow, repo, _ := newDeleteActionItemFixture(dao.ActionItem{ID: "root", SeriesID: "root", Timezone: "UTC"})
	repo.tombstoneErr = wantErr

	err := NewDeleteActionItemUseCase(uow, nil).Execute(context.Background(), "user-1", "task-1", "root")
	if !errors.Is(err, wantErr) {
		t.Errorf("Execute() error = %v, want %v", err, wantErr)
	}
	if repo.deleteFutureCall != 0 {
		t.Errorf("DeleteUneditedFutureBySeries() calls = %d, want 0 after tombstone failure", repo.deleteFutureCall)
	}
}

func TestDeleteActionItemUseCasePropagatesUOWErrors(t *testing.T) {
	t.Run("unit of work", func(t *testing.T) {
		wantErr := errors.New("transaction failed")
		uow := &deleteActionItemUOWFake{err: wantErr}

		err := NewDeleteActionItemUseCase(uow, nil).Execute(context.Background(), "user-1", "task-1", "item-1")
		if !errors.Is(err, wantErr) {
			t.Errorf("Execute() error = %v, want %v", err, wantErr)
		}
		if uow.calls != 1 || uow.callbackErr != nil {
			t.Errorf("UOW calls = %d and callback error = %v, want 1 call without callback", uow.calls, uow.callbackErr)
		}
	})
}
