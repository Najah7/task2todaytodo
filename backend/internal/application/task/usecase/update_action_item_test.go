package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

type updateActionItemUOWFake struct {
	repos Repositories
	err   error
	calls int
}

func (uow *updateActionItemUOWFake) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	uow.calls++
	if uow.err != nil {
		return uow.err
	}
	return fn(ctx, uow.repos)
}

type updateActionItemRepositoriesFake struct {
	taskProgressTestRepositories
	actionItems ActionItemRepository
}

func (repos updateActionItemRepositoriesFake) ActionItems() ActionItemRepository {
	return repos.actionItems
}

type updateActionItemRepositoryFake struct {
	ActionItemRepository
	current     dao.ActionItem
	updated     domain.ActionItem
	userID      domain.UserID
	taskID      domain.TaskID
	itemID      domain.ActionItemID
	getErr      error
	updateErr   error
	getCalls    int
	updateCalls int
}

func (*updateActionItemRepositoryFake) ReadTaskListProjection(_ context.Context, _ domain.UserID, taskIDs []string) (dao.TaskListProjectionSources, error) {
	items := make(map[string][]dao.ActionItem, len(taskIDs))
	for _, id := range taskIDs {
		items[id] = nil
	}
	return dao.TaskListProjectionSources{ActionItemsByTask: items}, nil
}

func (repo *updateActionItemRepositoryFake) GetForCommand(_ context.Context, userID domain.UserID, taskID domain.TaskID, itemID domain.ActionItemID, capability shared.Capability) (dao.ActionItem, error) {
	repo.getCalls++
	repo.userID, repo.taskID, repo.itemID = userID, taskID, itemID
	if capability != shared.ActionItemUpdate() {
		return dao.ActionItem{}, errors.New("unexpected capability")
	}
	return repo.current, repo.getErr
}

func (repo *updateActionItemRepositoryFake) UpdateForOwnedTask(_ context.Context, userID domain.UserID, item domain.ActionItem) (dao.ActionItem, error) {
	repo.updateCalls++
	repo.userID, repo.taskID, repo.updated = userID, item.TaskID, item
	if repo.updateErr != nil {
		return dao.ActionItem{}, repo.updateErr
	}
	return dao.ActionItem{ID: string(item.ID), TaskID: string(item.TaskID), Title: item.Title, Description: item.Description, DueDate: item.DueDate.Unix(), EstimatedMinutes: item.EstimatedMinutes, Priority: dao.Priority{Value: item.Priority.Value}, Completed: item.Completed, Position: item.Position}, nil
}

func newUpdateActionItemFixture() (*updateActionItemUOWFake, *updateActionItemRepositoryFake, *UpdateActionItemUseCase) {
	created := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	repo := &updateActionItemRepositoryFake{current: dao.ActionItem{
		ID: "item-1", TaskID: "task-1", Title: "Old title", Description: "Old description",
		DueDate: time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC).Unix(), EstimatedMinutes: intPointer(25), Priority: dao.Priority{Value: "medium"}, Position: 3,
		CreatedAt: created.Unix(), UpdatedAt: created.Unix(),
	}}
	uow := &updateActionItemUOWFake{repos: updateActionItemRepositoriesFake{actionItems: repo}}
	return uow, repo, NewUpdateActionItemUseCase(uow, nil)
}

func TestUpdateActionItemUseCaseUpdatesFieldsOnConcreteID(t *testing.T) {
	uow, repo, uc := newUpdateActionItemFixture()
	dueDate := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	estimate := 45
	got, err := uc.Execute(context.Background(), "user-1", "task-1", "item-1", patchValue("New title"), patchValue("New description"), patchValue(dueDate), ActionItemPlanningPatch{
		EstimatedMinutes: patchValue(estimate), Priority: patchValue("high"),
	})
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if repo.getCalls != 1 || repo.updateCalls != 1 || uow.calls != 1 {
		t.Fatalf("calls = UOW:%d get:%d update:%d; want one each", uow.calls, repo.getCalls, repo.updateCalls)
	}
	if repo.userID != "user-1" || repo.taskID != "task-1" || repo.itemID != "item-1" || repo.updated.ID != "item-1" {
		t.Errorf("repository scope / updated ID = %q/%q/%q/%q", repo.userID, repo.taskID, repo.itemID, repo.updated.ID)
	}
	if got.ID != "item-1" || got.Title != "New title" || got.Description != "New description" || got.DueDate != dueDate.Unix() || got.EstimatedMinutes == nil || *got.EstimatedMinutes != estimate || got.Priority.Value != "high" || got.Position != 3 {
		t.Errorf("updated item = %+v; want changed fields and preserved position", got)
	}
}

func TestUpdateActionItemUseCasePreservesFieldsOmittedFromPatch(t *testing.T) {
	_, repo, uc := newUpdateActionItemFixture()
	_, err := uc.Execute(context.Background(), "user-1", "task-1", "item-1", PatchField[string]{}, PatchField[string]{}, PatchField[time.Time]{})
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if repo.updated.Title != "Old title" || repo.updated.Description != "Old description" || repo.updated.EstimatedMinutes == nil || *repo.updated.EstimatedMinutes != 25 || repo.updated.Priority.Value != "medium" {
		t.Errorf("unchanged patch item = %+v; want persisted fields retained", repo.updated)
	}
}

func TestUpdateActionItemUseCaseRejectsNullTitleAndPropagatesNotFound(t *testing.T) {
	_, repo, uc := newUpdateActionItemFixture()
	if _, err := uc.Execute(context.Background(), "user-1", "task-1", "item-1", PatchField[string]{Present: true}, PatchField[string]{}, PatchField[time.Time]{}); !errors.Is(err, ErrActionItemPatchRequiredFieldNull) {
		t.Errorf("null title error = %v; want %v", err, ErrActionItemPatchRequiredFieldNull)
	}
	if repo.getCalls != 0 {
		t.Errorf("GetForCommand calls = %d; want 0 for invalid request", repo.getCalls)
	}

	wantErr := ErrActionItemNotFound
	repo.getErr = wantErr
	if _, err := uc.Execute(context.Background(), "user-1", "task-1", "missing", PatchField[string]{}, PatchField[string]{}, PatchField[time.Time]{}); !errors.Is(err, wantErr) {
		t.Errorf("missing item error = %v; want %v", err, wantErr)
	}
	if repo.updateCalls != 0 {
		t.Errorf("UpdateForOwnedTask calls = %d; want 0 for missing item", repo.updateCalls)
	}
}

func TestUpdateActionItemUseCasePropagatesRepositoryAndUOWErrors(t *testing.T) {
	wantErr := errors.New("repository unavailable")
	t.Run("repository", func(t *testing.T) {
		uow, repo, uc := newUpdateActionItemFixture()
		repo.updateErr = wantErr
		_, err := uc.Execute(context.Background(), "user-1", "task-1", "item-1", PatchField[string]{}, PatchField[string]{}, PatchField[time.Time]{})
		if !errors.Is(err, wantErr) {
			t.Errorf("Execute() error = %v, want %v", err, wantErr)
		}
		if uow.calls != 1 || repo.updateCalls != 1 {
			t.Errorf("calls = UOW:%d update:%d", uow.calls, repo.updateCalls)
		}
	})
	t.Run("unit of work", func(t *testing.T) {
		uow, repo, uc := newUpdateActionItemFixture()
		uow.err = wantErr
		_, err := uc.Execute(context.Background(), "user-1", "task-1", "item-1", PatchField[string]{}, PatchField[string]{}, PatchField[time.Time]{})
		if !errors.Is(err, wantErr) || repo.getCalls != 0 {
			t.Errorf("Execute() error = %v get calls=%d; want UOW error and no read", err, repo.getCalls)
		}
	})
}

func intPointer(value int) *int { return &value }

func patchValue[T any](value T) PatchField[T] { return PatchField[T]{Present: true, Value: &value} }
