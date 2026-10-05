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

type updateTodoItemUOWFake struct{ repos Repositories }

func (u *updateTodoItemUOWFake) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	return fn(ctx, u.repos)
}

type updateTodoItemRepositoriesFake struct {
	taskProgressTestRepositories
	items TodoItemRepository
}

func (r *updateTodoItemRepositoriesFake) TodoItems() TodoItemRepository { return r.items }

type fixedUpdateID string

func (id fixedUpdateID) Generate() string { return string(id) }

type updateTodoItemRepositoryFake struct {
	TodoItemRepository
	root                         dao.TodoItem
	current                      *dao.TodoItem
	updated                      domain.TodoItem
	result                       dao.TodoItem
	templateCalls, overrideCalls int
	snapshotID                   domain.TodoItemID
}

func (r *updateTodoItemRepositoryFake) GetForOwnedTask(context.Context, domain.UserID, domain.TaskID, domain.TodoItemID) (dao.TodoItem, error) {
	return r.root, nil
}
func (r *updateTodoItemRepositoryFake) ListByTask(context.Context, domain.UserID, domain.TaskID) ([]dao.TodoItem, error) {
	rows := []dao.TodoItem{r.root}
	if r.current != nil {
		rows = append(rows, *r.current)
	}
	return rows, nil
}
func (r *updateTodoItemRepositoryFake) UpsertTodoItemOverride(_ context.Context, _ domain.UserID, item domain.TodoItem) (string, error) {
	r.overrideCalls++
	r.updated = item
	return string(item.ID), nil
}
func (r *updateTodoItemRepositoryFake) UpdateTodoItemSeriesTemplate(_ context.Context, _ domain.UserID, _ domain.TaskID, _ domain.TodoItemID, snapshotID domain.TodoItemID, item domain.TodoItem) (dao.TodoItem, error) {
	r.templateCalls++
	r.snapshotID = snapshotID
	r.updated = item
	r.root.Title = item.Title
	return dao.TodoItem{ID: r.root.ID, TaskID: r.root.TaskID, SeriesID: r.root.ID, OccurrenceDate: r.root.OccurrenceDate, Title: item.Title, IntervalWeeks: r.root.IntervalWeeks, RepeatState: r.root.RepeatState, FrequencyAnchorDate: r.root.FrequencyAnchorDate, Timezone: r.root.Timezone}, nil
}
func (r *updateTodoItemRepositoryFake) UpdateForOwnedTask(_ context.Context, _ domain.UserID, item domain.TodoItem) (dao.TodoItem, error) {
	r.updated = item
	return dao.TodoItem{ID: string(item.ID), TaskID: string(item.TaskID), SeriesID: string(item.SeriesID), OccurrenceDate: item.OccurrenceDate.Format("2006-01-02"), Title: item.Title, Timezone: item.Timezone}, nil
}

type fixedUpdateTodayUOW struct{ Repositories }

func updateTodoItemRow(id string, interval int, state string) dao.TodoItem {
	return dao.TodoItem{ID: id, TaskID: "task-1", Title: "Review PR", Description: "Check tests", DueDate: mustParseDate("2026-10-05").Unix(), Position: 3, IntervalWeeks: interval, Frequencies: []dao.TaskFrequency{{Value: "mon"}}, RepeatState: state, FrequencyAnchorDate: mustParseDate("2026-10-05").Unix(), SeriesID: id, OccurrenceDate: "2026-10-05", Timezone: "UTC", CreatedAt: 100, UpdatedAt: 200}
}

func TestUpdateTodoItemFutureScopeSnapshotsRootAndKeepsCadenceAnchor(t *testing.T) {
	root := updateTodoItemRow("todo-root", 1, repeatStateActive)
	repo := &updateTodoItemRepositoryFake{root: root}
	uow := &updateTodoItemUOWFake{repos: &updateTodoItemRepositoriesFake{items: repo}}
	uc := NewUpdateTodoItemUseCase(uow, nil, fixedUpdateID("snapshot-id"))
	title := "new title"
	got, err := uc.ExecuteOccurrence(context.Background(), "user-1", "task-1", "todo-root", "2026-10-05", todoItemScopeFuture, PatchField[string]{Present: true, Value: &title}, PatchField[string]{}, PatchField[time.Time]{})
	if err != nil {
		t.Fatal(err)
	}
	if repo.templateCalls != 1 || repo.snapshotID != "snapshot-id" || repo.updated.Title != title {
		t.Fatalf("template calls=%d snapshot=%s title=%s", repo.templateCalls, repo.snapshotID, repo.updated.Title)
	}
	if repo.root.FrequencyAnchorDate != root.FrequencyAnchorDate || got.Title != title {
		t.Fatalf("root=%#v result=%#v", repo.root, got)
	}
}

func TestUpdateTodoItemFutureScopeRejectsDueDateEvenWhenCleared(t *testing.T) {
	root := updateTodoItemRow("todo-root", 1, repeatStateActive)
	repo := &updateTodoItemRepositoryFake{root: root}
	uow := &updateTodoItemUOWFake{repos: &updateTodoItemRepositoriesFake{items: repo}}
	uc := NewUpdateTodoItemUseCase(uow, nil, fixedUpdateID("snapshot-id"))

	for _, value := range []*time.Time{nil, func() *time.Time { due := mustParseDate("2026-10-12"); return &due }()} {
		_, err := uc.ExecuteOccurrence(context.Background(), "user-1", "task-1", "todo-root", "2026-10-05", todoItemScopeFuture, PatchField[string]{}, PatchField[string]{}, PatchField[time.Time]{Present: true, Value: value})
		if !errors.Is(err, ErrTodoItemFutureDueDateUnsupported) {
			t.Fatalf("future due_date PATCH error=%v, want %v", err, ErrTodoItemFutureDueDateUnsupported)
		}
	}
	if repo.templateCalls != 0 || repo.overrideCalls != 0 {
		t.Fatalf("future due_date PATCH mutated repository: template=%d override=%d", repo.templateCalls, repo.overrideCalls)
	}
}

func TestUpdateTodoItemCurrentFirstOccurrenceCreatesChildID(t *testing.T) {
	root := updateTodoItemRow("todo-root", 1, repeatStateActive)
	repo := &updateTodoItemRepositoryFake{root: root}
	uow := &updateTodoItemUOWFake{repos: &updateTodoItemRepositoriesFake{items: repo}}
	uc := NewUpdateTodoItemUseCase(uow, nil, fixedUpdateID("child-id"))
	title := "edited first occurrence"
	got, err := uc.ExecuteOccurrence(context.Background(), "user-1", "task-1", "todo-root", "2026-10-05", todoItemScopeCurrent, PatchField[string]{Present: true, Value: &title}, PatchField[string]{}, PatchField[time.Time]{})
	if err != nil {
		t.Fatal(err)
	}
	if repo.overrideCalls != 1 || got.ID != "child-id" || got.ID == root.ID || !repo.updated.IsException {
		t.Fatalf("first occurrence result=%#v child=%#v", got, repo.updated)
	}
}

func TestTodoItemDueDateOverrideSurvivesProjection(t *testing.T) {
	date := mustParseDate("2026-10-05")
	due := time.Date(2026, 10, 12, 16, 45, 0, 0, time.UTC)
	root := updateTodoItemRow("series", 1, repeatStateActive)
	exception := root
	exception.ID = "saved"
	exception.IsException = true
	exception.OccurrenceDate = "2026-10-12"
	exception.DueDate = due.Unix()
	item, err := todoDomainOccurrence(root, &exception, date.AddDate(0, 0, 7), "saved", false, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !item.DueDate.Equal(due) {
		t.Errorf("due date=%s want %s", item.DueDate, due)
	}
}

var _ shared.ID = fixedUpdateID("")
