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

type todoRuleUOWFake struct{ repos Repositories }

func (u todoRuleUOWFake) Do(ctx context.Context, fn func(context.Context, Repositories) error) error {
	return fn(ctx, u.repos)
}

type todoRuleReposFake struct {
	taskProgressTestRepositories
	todo  TodoItemRepository
	tasks TaskRepository
}

func (r todoRuleReposFake) TodoItems() TodoItemRepository { return r.todo }
func (r todoRuleReposFake) Tasks() TaskRepository {
	if r.tasks != nil {
		return r.tasks
	}
	return taskProgressTestRepository{}
}

type todoRuleRepositoryFake struct {
	TodoItemRepository
	root        dao.TodoItem
	anchor      time.Time
	interval    int
	frequencies []dao.TaskFrequency
	sets, stops int
}

func (r *todoRuleRepositoryFake) GetForOwnedTask(context.Context, domain.UserID, domain.TaskID, domain.TodoItemID) (dao.TodoItem, error) {
	return r.root, nil
}
func (r *todoRuleRepositoryFake) SetTodoItemRecurrence(_ context.Context, _ domain.UserID, _ domain.TaskID, _ domain.TodoItemID, anchor time.Time, interval int, frequencies []dao.TaskFrequency) error {
	r.anchor, r.interval, r.frequencies, r.sets = anchor, interval, frequencies, r.sets+1
	r.root.FrequencyAnchorDate = anchor.Unix()
	r.root.IntervalWeeks = interval
	r.root.Frequencies = frequencies
	r.root.RepeatState = repeatStateActive
	return nil
}
func (r *todoRuleRepositoryFake) StopTodoItemRecurrence(context.Context, domain.UserID, domain.TaskID, domain.TodoItemID) error {
	r.stops++
	r.root.RepeatState = repeatStateStopped
	r.root.IntervalWeeks = 0
	return nil
}

type todoFrequencyProgressFake struct {
	taskProgressTestRepository
	todo   *todoRuleRepositoryFake
	reads  []time.Time
	writes int
}

func (r *todoFrequencyProgressFake) LockByUserID(_ context.Context, userID domain.UserID, taskID domain.TaskID) (dao.Task, error) {
	return dao.Task{ID: string(taskID), UserID: string(userID), Status: dao.TaskStatus{Value: "done"}}, nil
}
func (r *todoFrequencyProgressFake) LockByUserIDWithPermission(ctx context.Context, userID domain.UserID, taskID domain.TaskID, _ shared.Capability) (dao.Task, error) {
	return r.LockByUserID(ctx, userID, taskID)
}
func (r *todoFrequencyProgressFake) SetStatusByUserID(_ context.Context, _ domain.UserID, _ domain.TaskID, status domain.TaskStatus) error {
	r.writes++
	if status.Value != "open" {
		return errors.New("expected task to reopen")
	}
	return nil
}
func (r *todoFrequencyProgressFake) SetStatusByUserIDWithPermission(ctx context.Context, userID domain.UserID, taskID domain.TaskID, status domain.TaskStatus, _ int32, _ shared.Capability) error {
	return r.SetStatusByUserID(ctx, userID, taskID, status)
}
func (r *todoFrequencyProgressFake) ReadTaskProgressSources(_ context.Context, taskIDs, _ []string, asOf time.Time) (dao.TaskProgressSources, error) {
	r.reads = append(r.reads, asOf)
	root := r.todo.root
	return dao.TaskProgressSources{
		Counts:   map[string]dao.TaskProgressCounts{},
		Statuses: map[string]dao.TaskStatus{string(taskIDs[0]): {Value: "done"}},
		TodoItemRoots: []dao.ProgressRecurrence{{
			TaskID: root.TaskID, SeriesID: root.SeriesID, OccurrenceDate: root.OccurrenceDate, Timezone: root.Timezone,
			RepeatState: root.RepeatState, FrequencyAnchorDate: root.FrequencyAnchorDate,
			IntervalWeeks: root.IntervalWeeks, Frequencies: root.Frequencies,
		}},
	}, nil
}

func TestUpdateTodoItemFrequencyAnchorsLocalToday(t *testing.T) {
	rootDate := mustParseDate("2026-01-05")
	repo := &todoRuleRepositoryFake{root: dao.TodoItem{ID: "todo-1", TaskID: "task-1", SeriesID: "todo-1", OccurrenceDate: rootDate.Format("2006-01-02"), Timezone: "Asia/Tokyo", Title: "run", IntervalWeeks: 1, Frequencies: []dao.TaskFrequency{{Value: "mon"}}, RepeatState: repeatStateActive}}
	uc := NewUpdateTodoItemFrequencyUseCase(todoRuleUOWFake{repos: todoRuleReposFake{todo: repo}}, nil)
	uc.now = func() time.Time { return time.Date(2026, 10, 5, 1, 0, 0, 0, time.UTC) }
	err := uc.Execute(context.Background(), UpdateTodoItemFrequencyInput{UserID: "user-1", TaskID: "task-1", TodoItemID: "todo-1", IntervalWeeks: 2, Frequencies: []string{"wed"}})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := repo.anchor.Format("2006-01-02"), "2026-10-05"; got != want {
		t.Fatalf("anchor=%s want %s", got, want)
	}
	if repo.sets != 1 || repo.interval != 2 || len(repo.frequencies) != 1 || repo.frequencies[0].Value != "wed" {
		t.Fatalf("set=%#v", repo)
	}
}

func TestUpdateTodoItemFrequencyStopPreservesAnchorAndRestartResetsIt(t *testing.T) {
	rootDate := mustParseDate("2026-01-05")
	repo := &todoRuleRepositoryFake{root: dao.TodoItem{ID: "todo-1", TaskID: "task-1", SeriesID: "todo-1", OccurrenceDate: rootDate.Format("2006-01-02"), Timezone: "Asia/Tokyo", Title: "run", IntervalWeeks: 1, RepeatState: repeatStateActive, FrequencyAnchorDate: rootDate.Unix()}}
	uc := NewUpdateTodoItemFrequencyUseCase(todoRuleUOWFake{repos: todoRuleReposFake{todo: repo}}, nil)
	uc.now = func() time.Time { return time.Date(2026, 10, 5, 1, 0, 0, 0, time.UTC) }
	if err := uc.Execute(context.Background(), UpdateTodoItemFrequencyInput{UserID: "user-1", TaskID: "task-1", TodoItemID: "todo-1"}); err != nil {
		t.Fatal(err)
	}
	if repo.stops != 1 || repo.root.RepeatState != repeatStateStopped || repo.root.FrequencyAnchorDate != rootDate.Unix() {
		t.Fatalf("stopped root=%#v", repo.root)
	}
	if err := uc.Execute(context.Background(), UpdateTodoItemFrequencyInput{UserID: "user-1", TaskID: "task-1", TodoItemID: "todo-1", IntervalWeeks: 1, Frequencies: []string{"mon"}}); err != nil {
		t.Fatal(err)
	}
	if repo.sets != 1 || repo.anchor.Format("2006-01-02") != "2026-10-05" {
		t.Fatalf("restart anchor=%v sets=%d", repo.anchor, repo.sets)
	}
}

func TestUpdateTodoItemFrequencyReopensDoneTaskWhenTodayVirtualOccurrenceAppears(t *testing.T) {
	rootDate := mustParseDate("2026-01-05")
	repo := &todoRuleRepositoryFake{root: dao.TodoItem{ID: "todo-1", TaskID: "task-1", SeriesID: "todo-1", OccurrenceDate: rootDate.Format("2006-01-02"), Timezone: "UTC", Title: "run", IntervalWeeks: 1, Frequencies: []dao.TaskFrequency{{Value: "tue"}}, RepeatState: repeatStateActive}}
	progress := &todoFrequencyProgressFake{todo: repo}
	fixedAsOf := time.Date(2026, 10, 5, 12, 34, 56, 123, time.UTC)
	uc := NewUpdateTodoItemFrequencyUseCase(todoRuleUOWFake{repos: todoRuleReposFake{todo: repo, tasks: progress}}, nil)
	uc.now = func() time.Time { return fixedAsOf }
	err := uc.Execute(context.Background(), UpdateTodoItemFrequencyInput{UserID: "user-1", TaskID: "task-1", TodoItemID: "todo-1", IntervalWeeks: 1, Frequencies: []string{"mon"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(progress.reads) != 2 || !progress.reads[0].Equal(fixedAsOf) || !progress.reads[1].Equal(fixedAsOf) {
		t.Fatalf("progress snapshot times=%v, want two snapshots at %v", progress.reads, fixedAsOf)
	}
	if progress.writes != 1 {
		t.Fatalf("status writes=%d, want task reopened once", progress.writes)
	}
}
