//go:build integration

package task_test

import (
	"sync"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	taskrepo "github.com/Najah7/task2todaytodo/internal/application/task/repository"
	"github.com/Najah7/task2todaytodo/internal/application/task/usecase"
)

func TestPersistedActionItemCompletionProjectsTaskProgress(t *testing.T) {
	pool := integrationPool(t)
	userID, taskID := seedTask(t, pool, "Asia/Tokyo")
	ctx := t.Context()
	uow := actionItemTestUOW{pool: pool}
	repo := taskrepo.NewActionItemRepository(pool)
	ids := []domain.ActionItemID{domain.ActionItemID(actionItemTestID{}.Generate()), domain.ActionItemID(actionItemTestID{}.Generate())}
	for _, id := range ids {
		item, err := domain.NewActionItem(id, taskID, "Persisted work")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := repo.CreateForOwnedTask(ctx, userID, item, true); err != nil {
			t.Fatalf("create persisted ActionItem: %v", err)
		}
	}

	page, err := usecase.NewListTasksUseCase(taskrepo.NewTaskRepository(pool), nil, repo).Execute(ctx, userID, usecase.CursorPageRequest{Size: 10})
	if err != nil || len(page.Items) != 1 || page.Items[0].ActionItemCount != 2 || page.Items[0].ActionItemCompletedCount != 0 || page.Items[0].Progress != 0 {
		t.Fatalf("initial task projection = %+v, error=%v; want two incomplete persisted rows", page.Items, err)
	}
	if err := usecase.NewCompleteActionItemUseCase(uow, nil).Execute(ctx, userID, taskID, ids[0]); err != nil {
		t.Fatalf("complete persisted ActionItem: %v", err)
	}
	page, err = usecase.NewListTasksUseCase(taskrepo.NewTaskRepository(pool), nil, repo).Execute(ctx, userID, usecase.CursorPageRequest{Size: 10})
	if err != nil || len(page.Items) != 1 || page.Items[0].ActionItemCount != 2 || page.Items[0].ActionItemCompletedCount != 1 || page.Items[0].Progress != 50 {
		t.Fatalf("completed task projection = %+v, error=%v; want one of two persisted rows complete", page.Items, err)
	}
}

func TestConcurrentFinalActionItemCompletionsSetTaskDone(t *testing.T) {
	pool := integrationPool(t)
	userID, taskID := seedTask(t, pool, "Asia/Tokyo")
	ctx := t.Context()
	repo := taskrepo.NewActionItemRepository(pool)
	ids := []domain.ActionItemID{domain.ActionItemID(actionItemTestID{}.Generate()), domain.ActionItemID(actionItemTestID{}.Generate())}
	for _, id := range ids {
		item, err := domain.NewActionItem(id, taskID, "Concurrent persisted work")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := repo.CreateForOwnedTask(ctx, userID, item, true); err != nil {
			t.Fatalf("create ActionItem: %v", err)
		}
	}

	uow := actionItemTestUOW{pool: pool}
	start := make(chan struct{})
	errCh := make(chan error, len(ids))
	var wg sync.WaitGroup
	for _, id := range ids {
		id := id
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errCh <- usecase.NewCompleteActionItemUseCase(uow, nil).Execute(ctx, userID, taskID, id)
		}()
	}
	close(start)
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Errorf("complete concurrent ActionItem: %v", err)
		}
	}
	status, err := taskrepo.NewTaskRepository(pool).GetByUserID(ctx, userID, taskID)
	if err != nil {
		t.Fatalf("read task after final completions: %v", err)
	}
	if status.Status.Value != "done" {
		t.Fatalf("task status after completing all persisted ActionItems = %q; want done", status.Status.Value)
	}
	page, err := usecase.NewListTasksUseCase(taskrepo.NewTaskRepository(pool), nil, repo).Execute(ctx, userID, usecase.CursorPageRequest{Size: 10})
	if err != nil || len(page.Items) != 1 || page.Items[0].Progress != 100 {
		t.Fatalf("completed task projection = %+v, error=%v; want 100%%", page.Items, err)
	}
}
