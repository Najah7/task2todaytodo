package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

type deleteActionItemUOWFake struct {
	repos Repositories
	err   error
	calls int
}

func (uow *deleteActionItemUOWFake) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	uow.calls++
	if uow.err != nil {
		return uow.err
	}
	return fn(ctx, uow.repos)
}

type deleteActionItemRepositoriesFake struct {
	taskProgressTestRepositories
	actionItems ActionItemRepository
}

func (repos deleteActionItemRepositoriesFake) ActionItems() ActionItemRepository {
	return repos.actionItems
}

type deleteActionItemRepositoryFake struct {
	ActionItemRepository
	item        dao.ActionItem
	getErr      error
	deleteErr   error
	userID      domain.UserID
	taskID      domain.TaskID
	itemID      domain.ActionItemID
	capability  shared.Capability
	getCalls    int
	deleteCalls int
}

func (*deleteActionItemRepositoryFake) ReadTaskListProjection(_ context.Context, _ domain.UserID, taskIDs []string) (dao.TaskListProjectionSources, error) {
	items := make(map[string][]dao.ActionItem, len(taskIDs))
	for _, id := range taskIDs {
		items[id] = nil
	}
	return dao.TaskListProjectionSources{ActionItemsByTask: items}, nil
}

func (*deleteActionItemRepositoryFake) CountForOwnedTask(context.Context, domain.UserID, domain.TaskID, shared.Capability) (int, error) {
	return 2, nil
}

func (repo *deleteActionItemRepositoryFake) GetForCommand(_ context.Context, userID domain.UserID, taskID domain.TaskID, itemID domain.ActionItemID, capability shared.Capability) (dao.ActionItem, error) {
	repo.getCalls++
	repo.userID, repo.taskID, repo.itemID, repo.capability = userID, taskID, itemID, capability
	if repo.getErr != nil {
		return dao.ActionItem{}, repo.getErr
	}
	return repo.item, nil
}

func (repo *deleteActionItemRepositoryFake) DeleteForOwnedTask(_ context.Context, userID domain.UserID, taskID domain.TaskID, itemID domain.ActionItemID) error {
	repo.deleteCalls++
	repo.userID, repo.taskID, repo.itemID = userID, taskID, itemID
	return repo.deleteErr
}

func newDeleteActionItemFixture(item dao.ActionItem) (*deleteActionItemUOWFake, *deleteActionItemRepositoryFake) {
	repo := &deleteActionItemRepositoryFake{item: item}
	uow := &deleteActionItemUOWFake{repos: deleteActionItemRepositoriesFake{actionItems: repo}}
	return uow, repo
}

func TestDeleteActionItemUseCaseDeletesOwnedConcreteItem(t *testing.T) {
	uow, repo := newDeleteActionItemFixture(dao.ActionItem{ID: "item-1", TaskID: "task-1", Title: "Action"})
	if err := NewDeleteActionItemUseCase(uow, nil).Execute(context.Background(), "user-1", "task-1", "item-1"); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if uow.calls != 1 || repo.getCalls != 1 || repo.deleteCalls != 1 {
		t.Fatalf("calls = UOW:%d get:%d delete:%d; want one each", uow.calls, repo.getCalls, repo.deleteCalls)
	}
	if repo.userID != "user-1" || repo.taskID != "task-1" || repo.itemID != "item-1" || repo.capability != shared.ActionItemDelete() {
		t.Errorf("repository scope = %q/%q/%q capability=%v", repo.userID, repo.taskID, repo.itemID, repo.capability)
	}
}

func TestDeleteActionItemUseCaseRejectsCompletedItemWithoutDeleting(t *testing.T) {
	uow, repo := newDeleteActionItemFixture(dao.ActionItem{ID: "item-1", Completed: true})
	err := NewDeleteActionItemUseCase(uow, nil).Execute(context.Background(), "user-1", "task-1", "item-1")
	if !errors.Is(err, ErrActionItemCompleted) || repo.deleteCalls != 0 {
		t.Fatalf("Execute() error=%v deletes=%d; want completed conflict and no delete", err, repo.deleteCalls)
	}
}

func TestDeleteActionItemUseCaseReturnsNotFoundAndPropagatesFailures(t *testing.T) {
	t.Run("missing item", func(t *testing.T) {
		uow, repo := newDeleteActionItemFixture(dao.ActionItem{ID: "item-1"})
		repo.getErr = ErrActionItemNotFound
		err := NewDeleteActionItemUseCase(uow, nil).Execute(context.Background(), "user-1", "task-1", "item-1")
		if !errors.Is(err, ErrActionItemNotFound) || repo.deleteCalls != 0 {
			t.Errorf("Execute() error=%v deletes=%d; want not found and no delete", err, repo.deleteCalls)
		}
	})
	t.Run("delete error", func(t *testing.T) {
		wantErr := errors.New("database unavailable")
		uow, repo := newDeleteActionItemFixture(dao.ActionItem{ID: "item-1"})
		repo.deleteErr = wantErr
		err := NewDeleteActionItemUseCase(uow, nil).Execute(context.Background(), "user-1", "task-1", "item-1")
		if !errors.Is(err, wantErr) || repo.deleteCalls != 1 {
			t.Errorf("Execute() error=%v deletes=%d; want original error after one delete", err, repo.deleteCalls)
		}
	})
	t.Run("unit of work", func(t *testing.T) {
		wantErr := errors.New("transaction failed")
		uow, repo := newDeleteActionItemFixture(dao.ActionItem{ID: "item-1"})
		uow.err = wantErr
		err := NewDeleteActionItemUseCase(uow, nil).Execute(context.Background(), "user-1", "task-1", "item-1")
		if !errors.Is(err, wantErr) || repo.getCalls != 0 {
			t.Errorf("Execute() error=%v gets=%d; want UOW error and no read", err, repo.getCalls)
		}
	})
}
