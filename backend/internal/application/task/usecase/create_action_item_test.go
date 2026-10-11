package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

type createActionItemUOWFake struct {
	repos Repositories
	err   error
	calls int
}

func (uow *createActionItemUOWFake) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	uow.calls++
	if uow.err != nil {
		return uow.err
	}
	return fn(ctx, uow.repos)
}

type createActionItemRepositoriesFake struct {
	taskProgressTestRepositories
	actionItems ActionItemRepository
}

func (repos createActionItemRepositoriesFake) ActionItems() ActionItemRepository {
	return repos.actionItems
}

type createActionItemRepositoryFake struct {
	ActionItemRepository
	created    domain.ActionItem
	appendTail bool
	err        error
}

func (*createActionItemRepositoryFake) ReadTaskListProjection(_ context.Context, _ domain.UserID, taskIDs []string) (dao.TaskListProjectionSources, error) {
	items := make(map[string][]dao.ActionItem, len(taskIDs))
	for _, id := range taskIDs {
		items[id] = nil
	}
	return dao.TaskListProjectionSources{ActionItemsByTask: items}, nil
}

func (repo *createActionItemRepositoryFake) CreateForOwnedTask(_ context.Context, _ domain.UserID, item domain.ActionItem, appendTail bool) (dao.ActionItem, error) {
	repo.created, repo.appendTail = item, appendTail
	if repo.err != nil {
		return dao.ActionItem{}, repo.err
	}
	return dao.ActionItem{ID: string(item.ID), TaskID: string(item.TaskID), Title: item.Title, Description: item.Description, Position: 4, DueDate: item.DueDate.Unix(), EstimatedMinutes: item.EstimatedMinutes, Priority: dao.Priority{Value: item.Priority.Value}}, nil
}

func newCreateActionItemFixture() (*createActionItemUOWFake, *createActionItemRepositoryFake, *CreateActionItemUseCase) {
	itemRepo := &createActionItemRepositoryFake{}
	uow := &createActionItemUOWFake{repos: createActionItemRepositoriesFake{actionItems: itemRepo}}
	return uow, itemRepo, NewCreateActionItemUseCase(uow, nil)
}

func TestCreateActionItemUseCaseCreatesOneConcreteItemAtTaskTail(t *testing.T) {
	uow, repo, uc := newCreateActionItemFixture()
	dueDate := time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)
	estimate := 35
	input := CreateActionItemInput{
		ID: "item-1", UserID: "user-1", TaskID: "task-1", Title: "Prepare release",
		Description: "Check changelog", DueDate: dueDate, EstimatedMinutes: &estimate, Priority: "high",
	}

	got, err := uc.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if got.ID != "item-1" || got.TaskID != "task-1" || got.Title != input.Title || got.Description != input.Description || got.Position != 4 || got.EstimatedMinutes == nil || *got.EstimatedMinutes != estimate || got.Priority.Value != "high" {
		t.Errorf("created DAO = %+v; want supplied single ActionItem fields", got)
	}
	if repo.created.ID != input.ID || repo.created.TaskID != input.TaskID || repo.created.Title != input.Title || repo.created.Description != input.Description || !repo.created.DueDate.Equal(dueDate) || repo.created.EstimatedMinutes == nil || *repo.created.EstimatedMinutes != estimate || repo.created.Priority.Value != "high" {
		t.Errorf("persisted domain item = %+v; want input fields", repo.created)
	}
	if !repo.appendTail || uow.calls != 1 {
		t.Errorf("appendTail=%t UOW calls=%d; want append at tail in one transaction", repo.appendTail, uow.calls)
	}
}

func TestCreateActionItemUseCaseRejectsInvalidInputBeforeCreate(t *testing.T) {
	_, repo, uc := newCreateActionItemFixture()
	_, err := uc.Execute(context.Background(), CreateActionItemInput{ID: "item-1", UserID: "user-1", TaskID: "task-1", Title: "   "})
	if !errors.Is(err, domain.ErrActionItemTitleEmpty) || repo.created.ID != "" {
		t.Errorf("Execute() error=%v created=%+v; want title validation and no write", err, repo.created)
	}
}

func TestCreateActionItemUseCasePropagatesRepositoryAndUOWErrors(t *testing.T) {
	wantErr := errors.New("database unavailable")
	t.Run("repository", func(t *testing.T) {
		uow, repo, uc := newCreateActionItemFixture()
		repo.err = wantErr
		if _, err := uc.Execute(context.Background(), CreateActionItemInput{ID: "item-1", UserID: "user-1", TaskID: "task-1", Title: "Item"}); !errors.Is(err, wantErr) {
			t.Errorf("Execute() error = %v, want %v", err, wantErr)
		}
		if uow.calls != 1 || repo.created.ID != "item-1" {
			t.Errorf("UOW calls=%d attempted item=%+v", uow.calls, repo.created)
		}
	})
	t.Run("unit of work", func(t *testing.T) {
		uow, repo, uc := newCreateActionItemFixture()
		uow.err = wantErr
		if _, err := uc.Execute(context.Background(), CreateActionItemInput{ID: "item-1", UserID: "user-1", TaskID: "task-1", Title: "Item"}); !errors.Is(err, wantErr) {
			t.Errorf("Execute() error = %v, want %v", err, wantErr)
		}
		if repo.created.ID != "" || uow.calls != 1 {
			t.Errorf("repository item=%+v UOW calls=%d; want no write in failed UOW", repo.created, uow.calls)
		}
	})
}
