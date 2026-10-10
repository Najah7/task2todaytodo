package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

type taskListRepositoryFake struct {
	candidates  []dao.Task
	page        []dao.Task
	pageRequest TaskListRequest
	pageLimit   int
}

func (repo *taskListRepositoryFake) ReadTaskListCandidates(context.Context, domain.UserID, TaskListRequest) ([]dao.Task, error) {
	return append([]dao.Task(nil), repo.candidates...), nil
}

func (repo *taskListRepositoryFake) ListTaskPage(_ context.Context, _ domain.UserID, request TaskListRequest, limit int) ([]dao.Task, error) {
	repo.pageRequest, repo.pageLimit = request, limit
	return append([]dao.Task(nil), repo.page...), nil
}

type taskListProjectionReaderFake struct{}

func (taskListProjectionReaderFake) ReadTaskListProjection(_ context.Context, _ domain.UserID, _ []string) (dao.TaskListProjectionSources, error) {
	return dao.TaskListProjectionSources{ActionItemsByTask: map[string][]dao.ActionItem{}, SkippedByTask: map[string]map[string]map[string]bool{}}, nil
}

func TestListTasksFilteredPageSummaryUsesStatusIndependentCandidateSet(t *testing.T) {
	zero := 0
	five := 5
	repo := &taskListRepositoryFake{
		candidates: []dao.Task{
			{ID: "open-zero", Status: dao.TaskStatus{Value: "open"}, ManualEstimatedMinutes: &zero},
			{ID: "open-five", Status: dao.TaskStatus{Value: "open"}, ManualEstimatedMinutes: &five},
			{ID: "doing", Status: dao.TaskStatus{Value: "in_progress"}, ManualEstimatedMinutes: &five},
			{ID: "done", Status: dao.TaskStatus{Value: "done"}},
		},
		page: []dao.Task{
			{ID: "open-zero", Status: dao.TaskStatus{Value: "open"}, ManualEstimatedMinutes: &zero},
			{ID: "open-five", Status: dao.TaskStatus{Value: "open"}, ManualEstimatedMinutes: &five},
		},
	}
	useCase := &ListTasksUseCase{repo: repo, projection: taskListProjectionReaderFake{}}
	page, err := useCase.ExecuteFilteredPage(context.Background(), "user-1", TaskListRequest{
		Size: 2, Status: "open", DueFilter: "all", SortBy: "due_date", SortOrder: "asc",
		AsOf: time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.Summary.TotalCount != 2 || page.Summary.StatusCounts["open"] != 2 || page.Summary.StatusCounts["in_progress"] != 1 || page.Summary.StatusCounts["done"] != 1 {
		t.Fatalf("summary counts=%+v; want status-filtered total and unfiltered status tabs", page.Summary)
	}
	if page.Summary.EstimatedMinutesTotal == nil || *page.Summary.EstimatedMinutesTotal != 5 {
		t.Fatalf("estimate summary=%+v; want explicit zero retained in 0+5", page.Summary.EstimatedMinutesTotal)
	}
	if page.Summary.ActionItemTotalCount != 0 || page.Summary.ActionItemCompletedCount != 0 {
		t.Fatalf("child summary=%+v; want no ActionItems", page.Summary)
	}
	if len(page.Items) != 2 || page.Items[0].EstimatedMinutes == nil || *page.Items[0].EstimatedMinutes != 0 || repo.pageLimit != 3 || repo.pageRequest.Status != "open" {
		t.Fatalf("page=%+v request=%+v limit=%d", page, repo.pageRequest, repo.pageLimit)
	}
}

func TestListTasksFilteredPageRejectsInvalidSort(t *testing.T) {
	useCase := NewListTasksUseCase(nil, nil)
	_, err := useCase.ExecuteFilteredPage(context.Background(), "user-1", TaskListRequest{Size: 10, SortBy: "progress"})
	if err != ErrInvalidTaskListRequest {
		t.Fatalf("invalid sort error=%v; want %v", err, ErrInvalidTaskListRequest)
	}
}

func TestTaskEstimateUsesVisibleSavedHistoryAfterSeriesRootDeletion(t *testing.T) {
	manual, child := 99, 7
	root := recurringActionItemRoot()
	root.Deleted = true
	saved := root
	saved.ID = "saved-occurrence"
	saved.Deleted = false
	saved.IsException = true
	saved.OccurrenceDate = "2026-10-05"
	saved.Completed = true
	saved.EstimatedMinutes = &child
	tasks, err := ApplyTaskListProjection([]dao.Task{{
		ID: "task", Status: dao.TaskStatus{Value: "open"}, ManualEstimatedMinutes: &manual,
	}}, dao.TaskListProjectionSources{
		ActionItemsByTask: map[string][]dao.ActionItem{"task": {root, saved}},
		SkippedByTask:     map[string]map[string]map[string]bool{"task": {}},
	}, time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if tasks[0].EstimateSource != "action_items" || tasks[0].EstimatedMinutes == nil || *tasks[0].EstimatedMinutes != child {
		t.Fatalf("estimate source/value = %q/%v; want visible saved row sum %d", tasks[0].EstimateSource, tasks[0].EstimatedMinutes, child)
	}
}
