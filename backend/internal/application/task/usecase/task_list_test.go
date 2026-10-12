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

type taskListProjectionReaderFake struct {
	sources dao.TaskListProjectionSources
}

func (reader taskListProjectionReaderFake) ReadTaskListProjection(_ context.Context, _ domain.UserID, _ []string) (dao.TaskListProjectionSources, error) {
	return reader.sources, nil
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
	useCase := &ListTasksUseCase{repo: repo, projection: taskListProjectionReaderFake{sources: dao.TaskListProjectionSources{ActionItemsByTask: map[string][]dao.ActionItem{}}}}
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

func TestFilteredTaskPagePreservesCapabilitiesForSelfStartedAndOtherAssignee(t *testing.T) {
	asOf := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	makeTask := func(id, title, assignee string) dao.Task {
		return dao.Task{
			ID: id, Title: title, AssigneeID: assignee,
			Status: dao.TaskStatus{Value: "open"}, CanUpdate: true, CanAssign: true,
			CursorCreatedAt: "candidate-cursor",
		}
	}
	repo := &taskListRepositoryFake{
		candidates: []dao.Task{
			makeTask("self", "candidate self", "user-1"),
			makeTask("started", "candidate started", "user-1"),
			makeTask("other", "candidate other", "user-2"),
		},
		page: []dao.Task{
			func() dao.Task {
				task := makeTask("self", "page self", "user-1")
				task.CursorCreatedAt = "page-cursor"
				return task
			}(),
			makeTask("started", "page started", "user-1"),
			makeTask("other", "page other", "user-2"),
		},
	}
	useCase := &ListTasksUseCase{
		repo: repo,
		projection: taskListProjectionReaderFake{sources: dao.TaskListProjectionSources{
			ActionItemsByTask:    map[string][]dao.ActionItem{},
			StartedTasks:         map[string]bool{"started": true},
			CanCreateActionItems: map[string]bool{"self": true, "started": true, "other": true},
		}},
	}
	page, err := useCase.ExecuteFilteredPage(context.Background(), "user-1", TaskListRequest{
		Size: 3, Status: "all", DueFilter: "all", SortBy: "created_at", SortOrder: "desc", AsOf: asOf,
	})
	if err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]dao.Task, len(page.Items))
	for _, item := range page.Items {
		byID[item.ID] = item
	}
	self := byID["self"]
	if !self.CanAddToToday || !self.CanAddActionItem || !self.EstimateEditable || !self.CanAssign {
		t.Errorf("unstarted self-assigned Task lost capabilities in filtered page: %+v", self)
	}
	if self.Title != "page self" || self.CursorCreatedAt != "page-cursor" {
		t.Errorf("filtered page did not preserve displayed/sort fields: title=%q cursor=%q", self.Title, self.CursorCreatedAt)
	}
	started := byID["started"]
	if !started.Started || !started.CanAddToToday || started.EstimateEditable || started.CanAddActionItem {
		t.Errorf("started Task capabilities in filtered page = %+v; want manual Today add allowed but estimate/child add locked", started)
	}
	other := byID["other"]
	if other.CanAddToToday {
		t.Errorf("other-assignee Task can_add_to_today=%t; want false", other.CanAddToToday)
	}
}

func TestTaskListProgressAndEstimateUsePersistedActionItems(t *testing.T) {
	manual, child := 99, 7
	other := 12
	tasks, err := ApplyTaskListProjection([]dao.Task{{
		ID: "task", Status: dao.TaskStatus{Value: "open"}, ManualEstimatedMinutes: &manual,
	}}, dao.TaskListProjectionSources{
		ActionItemsByTask: map[string][]dao.ActionItem{"task": {
			{ID: "item-1", TaskID: "task", EstimatedMinutes: &child, Completed: true},
			{ID: "item-2", TaskID: "task", EstimatedMinutes: &other},
		}},
	}, time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if tasks[0].ActionItemCount != 2 || tasks[0].ActionItemCompletedCount != 1 || tasks[0].Progress != 50 {
		t.Fatalf("ActionItem count/completed/progress = %d/%d/%d; want 2/1/50", tasks[0].ActionItemCount, tasks[0].ActionItemCompletedCount, tasks[0].Progress)
	}
	if tasks[0].EstimateSource != "action_items" || tasks[0].EstimatedMinutes == nil || *tasks[0].EstimatedMinutes != child+other {
		t.Fatalf("estimate source/value = %q/%v; want persisted ActionItem sum %d", tasks[0].EstimateSource, tasks[0].EstimatedMinutes, child+other)
	}
}
