package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

type reorderActionItemUOWFake struct {
	repos Repositories
	calls int
	err   error
}

func (uow *reorderActionItemUOWFake) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	uow.calls++
	if uow.err != nil {
		return uow.err
	}
	return fn(ctx, uow.repos)
}

type reorderActionItemRepositoriesFake struct {
	taskProgressTestRepositories
	items ActionItemRepository
	tasks TaskRepository
}

func (repos reorderActionItemRepositoriesFake) ActionItems() ActionItemRepository { return repos.items }
func (repos reorderActionItemRepositoriesFake) Tasks() TaskRepository             { return repos.tasks }

type reorderActionItemRepositoryFake struct {
	ActionItemRepository
	rows         []dao.ActionItem
	ownerID      domain.UserID
	reorderErr   error
	reorderCalls int
	gotUserID    domain.UserID
	gotTaskID    domain.TaskID
	gotItemID    domain.ActionItemID
	gotPosition  int
}

func (repo *reorderActionItemRepositoryFake) ReadTaskListProjection(_ context.Context, _ domain.UserID, taskIDs []string) (dao.TaskListProjectionSources, error) {
	items := make(map[string][]dao.ActionItem, len(taskIDs))
	for _, taskID := range taskIDs {
		for _, item := range repo.rows {
			if item.TaskID == taskID && !item.Deleted {
				items[taskID] = append(items[taskID], item)
			}
		}
	}
	return dao.TaskListProjectionSources{ActionItemsByTask: items}, nil
}

func (repo *reorderActionItemRepositoryFake) ReorderForOwnedTask(_ context.Context, userID domain.UserID, taskID domain.TaskID, id domain.ActionItemID, position int) (dao.ActionItem, error) {
	repo.reorderCalls++
	repo.gotUserID, repo.gotTaskID, repo.gotItemID, repo.gotPosition = userID, taskID, id, position
	if repo.reorderErr != nil {
		return dao.ActionItem{}, repo.reorderErr
	}
	if userID != repo.ownerID {
		return dao.ActionItem{}, ErrActionItemNotFound
	}
	indices := make([]int, 0)
	target := -1
	for index, item := range repo.rows {
		if item.TaskID == string(taskID) && !item.Deleted {
			indices = append(indices, index)
			if item.ID == string(id) {
				target = len(indices) - 1
			}
		}
	}
	if target < 0 {
		return dao.ActionItem{}, ErrActionItemNotFound
	}
	if position < 0 || position >= len(indices) {
		return dao.ActionItem{}, ErrActionItemPositionOutOfRange
	}
	ordered := make([]dao.ActionItem, len(indices))
	for i, index := range indices {
		ordered[i] = repo.rows[index]
	}
	targetItem := ordered[target]
	ordered = append(ordered[:target], ordered[target+1:]...)
	ordered = append(ordered, dao.ActionItem{})
	copy(ordered[position+1:], ordered[position:])
	ordered[position] = targetItem
	for i, index := range indices {
		ordered[i].Position = i
		repo.rows[index] = ordered[i]
	}
	for _, item := range repo.rows {
		if item.ID == string(id) {
			return item, nil
		}
	}
	return dao.ActionItem{}, ErrActionItemNotFound
}

type reorderProgressFake struct {
	taskProgressTestRepository
	statusWrites int
}

func (repo *reorderProgressFake) LockByUserID(_ context.Context, userID domain.UserID, taskID domain.TaskID) (dao.Task, error) {
	return dao.Task{ID: string(taskID), UserID: string(userID), Status: dao.TaskStatus{Value: "open"}}, nil
}

func (repo *reorderProgressFake) SetStatusByUserID(_ context.Context, _ domain.UserID, _ domain.TaskID, _ domain.TaskStatus) error {
	repo.statusWrites++
	return nil
}

func newReorderActionItemFixture() (*reorderActionItemUOWFake, *reorderActionItemRepositoryFake, *reorderProgressFake, *ReorderActionItemUseCase) {
	repo := &reorderActionItemRepositoryFake{ownerID: "user-1", rows: []dao.ActionItem{
		{ID: "item-a", TaskID: "task-1", Title: "A", Position: 0},
		{ID: "item-b", TaskID: "task-1", Title: "B", Position: 1},
		{ID: "item-c", TaskID: "task-1", Title: "C", Position: 2},
		{ID: "other-task-item", TaskID: "task-2", Title: "Other task", Position: 0},
	}}
	progress := &reorderProgressFake{}
	uow := &reorderActionItemUOWFake{repos: reorderActionItemRepositoriesFake{items: repo, tasks: progress}}
	return uow, repo, progress, NewReorderActionItemUseCase(uow, nil)
}

func TestReorderActionItemUseCaseReordersConcreteItemsOnTask(t *testing.T) {
	uow, repo, _, useCase := newReorderActionItemFixture()
	got, err := useCase.Execute(context.Background(), "user-1", "task-1", "item-c", 0)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if got.ID != "item-c" || got.Position != 0 || repo.reorderCalls != 1 || uow.calls != 1 {
		t.Errorf("result=%+v calls=%d/%d; want item-c at position 0 and one repository/UOW call", got, repo.reorderCalls, uow.calls)
	}
	if gotIDs := reorderedActionItemIDs(repo.rows, "task-1"); len(gotIDs) != 3 || gotIDs[0] != "item-c" || gotIDs[1] != "item-a" || gotIDs[2] != "item-b" {
		t.Errorf("task item order = %v; want [item-c item-a item-b]", gotIDs)
	}
	if gotIDs := reorderedActionItemIDs(repo.rows, "task-2"); len(gotIDs) != 1 || gotIDs[0] != "other-task-item" {
		t.Errorf("other task item order = %v; want unchanged", gotIDs)
	}
	if repo.gotUserID != "user-1" || repo.gotTaskID != "task-1" || repo.gotItemID != "item-c" || repo.gotPosition != 0 {
		t.Errorf("repository arguments=%q/%q/%q/%d", repo.gotUserID, repo.gotTaskID, repo.gotItemID, repo.gotPosition)
	}
}

func TestReorderActionItemUseCaseRejectsOutOfRangeAndPreservesTaskStatus(t *testing.T) {
	_, repo, progress, useCase := newReorderActionItemFixture()
	if _, err := useCase.Execute(context.Background(), "user-1", "task-1", "item-a", 3); !errors.Is(err, ErrActionItemPositionOutOfRange) {
		t.Fatalf("out-of-range error = %v; want %v", err, ErrActionItemPositionOutOfRange)
	}
	if repo.reorderCalls != 1 {
		t.Fatalf("reorder calls = %d; want one validation call", repo.reorderCalls)
	}
	if progress.statusWrites != 0 {
		t.Errorf("status writes = %d; want no status change for reorder", progress.statusWrites)
	}
}

func TestReorderActionItemUseCaseRejectsNegativePositionBeforeTransaction(t *testing.T) {
	uow, repo, _, useCase := newReorderActionItemFixture()
	if _, err := useCase.Execute(context.Background(), "user-1", "task-1", "item-a", -1); !errors.Is(err, ErrActionItemPositionOutOfRange) {
		t.Fatalf("negative position error = %v; want %v", err, ErrActionItemPositionOutOfRange)
	}
	if uow.calls != 0 || repo.reorderCalls != 0 {
		t.Errorf("UOW/repository calls=%d/%d; want no calls for invalid position", uow.calls, repo.reorderCalls)
	}
}

func TestReorderActionItemUseCasePropagatesRepositoryAndUOWErrors(t *testing.T) {
	wantErr := errors.New("database unavailable")
	t.Run("repository", func(t *testing.T) {
		uow, repo, _, useCase := newReorderActionItemFixture()
		repo.reorderErr = wantErr
		if _, err := useCase.Execute(context.Background(), "user-1", "task-1", "item-a", 1); !errors.Is(err, wantErr) {
			t.Errorf("Execute() error = %v, want %v", err, wantErr)
		}
		if uow.calls != 1 || repo.reorderCalls != 1 {
			t.Errorf("UOW/repository calls=%d/%d", uow.calls, repo.reorderCalls)
		}
	})
	t.Run("unit of work", func(t *testing.T) {
		uow, repo, _, useCase := newReorderActionItemFixture()
		uow.err = wantErr
		if _, err := useCase.Execute(context.Background(), "user-1", "task-1", "item-a", 1); !errors.Is(err, wantErr) {
			t.Errorf("Execute() error = %v, want %v", err, wantErr)
		}
		if repo.reorderCalls != 0 {
			t.Errorf("repository calls=%d; want none after UOW failure", repo.reorderCalls)
		}
	})
}

func reorderedActionItemIDs(items []dao.ActionItem, taskID string) []string {
	selected := make([]dao.ActionItem, 0)
	for _, item := range items {
		if item.TaskID == taskID && !item.Deleted {
			selected = append(selected, item)
		}
	}
	for i := range selected {
		for j := i + 1; j < len(selected); j++ {
			if selected[j].Position < selected[i].Position {
				selected[i], selected[j] = selected[j], selected[i]
			}
		}
	}
	ids := make([]string, len(selected))
	for i, item := range selected {
		ids[i] = item.ID
	}
	return ids
}

func (repo *reorderProgressFake) ReadTaskProgressSources(_ context.Context, taskIDs []string, _ time.Time) (dao.TaskProgressSources, error) {
	counts := make(map[string]dao.TaskProgressCounts, len(taskIDs))
	statuses := make(map[string]dao.TaskStatus, len(taskIDs))
	for _, id := range taskIDs {
		counts[id] = dao.TaskProgressCounts{Total: 3, Completed: 0}
		statuses[id] = dao.TaskStatus{Value: "open"}
	}
	return dao.TaskProgressSources{Counts: counts, Statuses: statuses}, nil
}
