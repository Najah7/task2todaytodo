package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared/pagination"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

var ErrInvalidTaskListRequest = errors.New("invalid task list request")

type TaskListRequest struct {
	Size      int
	Status    string
	ProjectID string
	DueFilter string
	Title     string
	SortBy    string
	SortOrder string
	Anchor    *TaskListCursor
	AsOf      time.Time
}

type TaskListCursor struct {
	ID          string
	CreatedAt   string
	Direction   string
	Title       string
	DueDate     string
	DueDateNull bool
}

type TaskListSummary struct {
	TotalCount               int
	StatusCounts             map[string]int
	ActionItemTotalCount     int
	ActionItemCompletedCount int
	EstimatedMinutesTotal    *int
}

type TaskListPage struct {
	Items    []dao.Task
	Next     *TaskListCursor
	Previous *TaskListCursor
	Summary  TaskListSummary
}

type taskListRepository interface {
	ListTaskPage(context.Context, domain.UserID, TaskListRequest, int) ([]dao.Task, error)
	ReadTaskListCandidates(context.Context, domain.UserID, TaskListRequest) ([]dao.Task, error)
}

func (uc *ListTasksUseCase) ExecuteFilteredPage(ctx context.Context, userID domain.UserID, request TaskListRequest) (output TaskListPage, err error) {
	defer func() { logUnexpectedTaskFailure(uc.logger, ctx, "ListTasksUseCase.ExecuteFilteredPage", err) }()
	if err := validateTaskListRequest(userID, &request); err != nil {
		return TaskListPage{}, err
	}
	if uc.repo == nil || uc.projection == nil {
		return TaskListPage{}, ErrInvalidTaskListRequest
	}
	all, err := uc.repo.ReadTaskListCandidates(ctx, userID, request)
	if err != nil {
		return TaskListPage{}, err
	}
	all, err = EnrichTaskList(ctx, uc.projection, userID, all, request.AsOf)
	if err != nil {
		return TaskListPage{}, err
	}
	statusCounts := map[string]int{"open": 0, "in_progress": 0, "pending": 0, "waiting_on_others": 0, "done": 0}
	enrichedByID := make(map[string]dao.Task, len(all))
	summary := TaskListSummary{StatusCounts: statusCounts}
	for _, task := range all {
		enrichedByID[task.ID] = task
		if _, ok := statusCounts[task.Status.Value]; ok {
			statusCounts[task.Status.Value]++
		}
		if request.Status != "all" && task.Status.Value != request.Status {
			continue
		}
		summary.TotalCount++
		summary.ActionItemTotalCount += task.ActionItemCount
		summary.ActionItemCompletedCount += task.ActionItemCompletedCount
		if task.EstimatedMinutes != nil {
			if summary.EstimatedMinutesTotal == nil {
				zero := 0
				summary.EstimatedMinutesTotal = &zero
			}
			*summary.EstimatedMinutesTotal += *task.EstimatedMinutes
		}
	}
	direction := "forward"
	if request.Anchor != nil {
		direction = request.Anchor.Direction
		if direction != "forward" && direction != "backward" {
			return TaskListPage{}, ErrInvalidTaskListRequest
		}
	}
	pageRows, err := uc.repo.ListTaskPage(ctx, userID, request, request.Size+1)
	if err != nil {
		return TaskListPage{}, err
	}
	for i := range pageRows {
		if enriched, ok := enrichedByID[pageRows[i].ID]; ok {
			pageRows[i].Progress = enriched.Progress
			pageRows[i].EstimatedMinutes = enriched.EstimatedMinutes
			pageRows[i].EstimateSource = enriched.EstimateSource
			pageRows[i].ActionItemCount = enriched.ActionItemCount
			pageRows[i].ActionItemCompletedCount = enriched.ActionItemCompletedCount
			pageRows[i].Status = enriched.Status
		}
	}
	items := pageRows
	more := len(items) > request.Size
	if more {
		items = items[:request.Size]
	}
	if direction == "backward" {
		reverseTaskRows(items)
	}
	var next, previous *TaskListCursor
	if len(items) > 0 {
		if (direction == "forward" && more) || direction == "backward" {
			cursor := taskListCursor(items[len(items)-1], request.SortBy, "forward")
			next = &cursor
		}
		if request.Anchor != nil && (direction == "forward" || more) {
			cursor := taskListCursor(items[0], request.SortBy, "backward")
			previous = &cursor
		}
	}
	return TaskListPage{Items: items, Next: next, Previous: previous, Summary: summary}, nil
}

func reverseTaskRows(rows []dao.Task) {
	for left, right := 0, len(rows)-1; left < right; left, right = left+1, right-1 {
		rows[left], rows[right] = rows[right], rows[left]
	}
}

func validateTaskListRequest(userID domain.UserID, request *TaskListRequest) error {
	if userID == "" || request.Size < 1 || request.Size > pagination.MaxPageSize {
		return ErrInvalidTaskListRequest
	}
	if request.Status == "" {
		request.Status = "all"
	}
	if request.Status != "all" {
		if _, err := domain.NewTaskStatus(request.Status); err != nil {
			return ErrInvalidTaskListRequest
		}
	}
	if request.DueFilter == "" {
		request.DueFilter = "all"
	}
	switch request.DueFilter {
	case "all", "overdue", "today", "due_soon", "no_due":
	default:
		return ErrInvalidTaskListRequest
	}
	if request.SortBy == "" {
		request.SortBy = "due_date"
	}
	if request.SortOrder == "" {
		if request.SortBy == "created_at" {
			request.SortOrder = "desc"
		} else {
			request.SortOrder = "asc"
		}
	}
	if (request.SortBy != "due_date" && request.SortBy != "title" && request.SortBy != "created_at") || (request.SortOrder != "asc" && request.SortOrder != "desc") {
		return ErrInvalidTaskListRequest
	}
	request.Title = strings.TrimSpace(request.Title)
	if request.AsOf.IsZero() {
		request.AsOf = time.Now().UTC()
	}
	if request.Anchor != nil && (request.Anchor.ID == "" || request.Anchor.CreatedAt == "") {
		return ErrInvalidTaskListRequest
	}
	return nil
}

func taskListCursor(task dao.Task, sortBy, direction string) TaskListCursor {
	cursor := TaskListCursor{ID: task.ID, CreatedAt: task.CursorCreatedAt, Direction: direction}
	switch sortBy {
	case "title":
		cursor.Title = strings.ToLower(task.Title)
	case "due_date":
		cursor.DueDateNull = task.DueDate == 0
		if !cursor.DueDateNull {
			cursor.DueDate = time.Unix(task.DueDate, 0).UTC().Format("2006-01-02")
		}
	}
	return cursor
}
