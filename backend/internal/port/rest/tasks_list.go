package rest

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	taskusecase "github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/Najah7/task2todaytodo/internal/port/rest/pagination"
)

// List returns filtered Tasks and summaries for the full filtered result set.
//
//	@Summary		List tasks
//	@Description	Returns a filtered page of tasks with counts and estimates for the full matching result set.
//	@Tags			Tasks
//	@Produce		json
//	@Security		BearerAuth
//	@Param			status		query		string	false	"Task status"	Enums(all,open,in_progress,pending,waiting_on_others,done)
//	@Param			project_id	query		string	false	"Project ID"
//	@Param			due_filter	query		string	false	"Due date filter"	Enums(all,overdue,today,due_soon,no_due)
//	@Param			title		query		string	false	"Title substring"
//	@Param			sort_by		query		string	false	"Sort field"		Enums(due_date,title,created_at)
//	@Param			sort_order	query		string	false	"Sort direction"	Enums(asc,desc)
//	@Param			page_size	query		int		false	"Items per page (default 50, maximum 100)"
//	@Param			page_token	query		string	false	"Opaque cursor for the next page"
//	@Param			fields		query		string	false	"Response field mask"
//	@Success		200			{object}	TaskListResponse
//	@Failure		400			{object}	ErrResponse	"Invalid filter or pagination"
//	@Failure		401			{object}	ErrResponse	"Unauthorized"
//	@Failure		500			{object}	ErrResponse	"Failed to list tasks"
//	@Router			/tasks [get]
func (h *TaskHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.userID(r)
	if !ok {
		writeUnauthorized(w, errSpecTasksListFailed)
		return
	}
	query := r.URL.Query()
	for _, key := range []string{"status", "project_id", "due_filter", "title", "sort_by", "sort_order"} {
		if len(query[key]) > 1 {
			writeListError(w, errSpecTasksListFailed, invalidListRequestError{field: key})
			return
		}
	}
	status := query.Get("status")
	if status == "" {
		status = "all"
	}
	projectID := query.Get("project_id")
	dueFilter := query.Get("due_filter")
	if dueFilter == "" {
		dueFilter = "all"
	}
	title := query.Get("title")
	sortBy := query.Get("sort_by")
	if sortBy == "" {
		sortBy = "due_date"
	}
	sortOrder := query.Get("sort_order")
	if sortOrder == "" {
		if sortBy == "created_at" {
			sortOrder = "desc"
		} else {
			sortOrder = "asc"
		}
	}
	if status != "all" {
		if _, err := domain.NewTaskStatus(status); err != nil {
			writeListError(w, errSpecTasksListFailed, invalidListRequestError{field: "status"})
			return
		}
	}
	switch dueFilter {
	case "all", "overdue", "today", "due_soon", "no_due":
	default:
		writeListError(w, errSpecTasksListFailed, invalidListRequestError{field: "due_filter"})
		return
	}
	if sortBy != "due_date" && sortBy != "title" && sortBy != "created_at" {
		writeListError(w, errSpecTasksListFailed, invalidListRequestError{field: "sort_by"})
		return
	}
	if sortOrder != "asc" && sortOrder != "desc" {
		writeListError(w, errSpecTasksListFailed, invalidListRequestError{field: "sort_order"})
		return
	}
	order := fmt.Sprintf("status=%s|project_id=%s|due_filter=%s|title=%s|sort=%s|direction=%s", status, projectID, dueFilter, url.QueryEscape(title), sortBy, sortOrder)
	parsed, err := parseListRequest(r, h.pageTokens, string(userID), "tasks", "", order, TaskListResponse{})
	if err != nil {
		writeListError(w, errSpecTasksListFailed, err)
		return
	}
	asOf := parsed.AsOf
	if asOf.IsZero() {
		asOf = time.Now().UTC()
	}
	var cursor *taskusecase.TaskListCursor
	if parsed.Anchor != nil {
		cursor = &taskusecase.TaskListCursor{ID: parsed.Anchor.ID, CreatedAt: parsed.Anchor.At, Direction: parsed.Anchor.Direction, Title: parsed.Anchor.SortValue, DueDate: parsed.Anchor.SortValue, DueDateNull: parsed.Anchor.SortValueNull}
	}
	page, err := h.tasks.List.ExecuteFilteredPage(r.Context(), userID, taskusecase.TaskListRequest{
		Size: parsed.Size, Status: status, ProjectID: projectID, DueFilter: dueFilter,
		Title: title, SortBy: sortBy, SortOrder: sortOrder, Anchor: cursor, AsOf: asOf,
	})
	if err != nil {
		if errors.Is(err, taskusecase.ErrInvalidTaskListRequest) {
			writeListError(w, errSpecTasksListFailed, err)
			return
		}
		writeTaskError(w, errSpecTasksListFailed, err)
		return
	}
	items := make([]TaskResponse, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, taskResponse(item))
	}
	response := TaskListResponse{
		Items: items, NextPageToken: "", TotalCount: page.Summary.TotalCount,
		StatusCounts:             page.Summary.StatusCounts,
		ActionItemTotalCount:     page.Summary.ActionItemTotalCount,
		ActionItemCompletedCount: page.Summary.ActionItemCompletedCount,
		EstimatedMinutesTotal:    page.Summary.EstimatedMinutesTotal,
	}
	if page.Next != nil {
		token, err := encodeTaskListCursor(h.pageTokens, parsed.Scope, page.Next, sortBy, asOf)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, errSpecTasksListFailed, ErrDetailInternalServerError)
			return
		}
		response.NextPageToken = token
	}
	if page.Previous != nil {
		token, err := encodeTaskListCursor(h.pageTokens, parsed.Scope, page.Previous, sortBy, asOf)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, errSpecTasksListFailed, ErrDetailInternalServerError)
			return
		}
		response.PreviousPageToken = token
	}
	encoded, err := parsed.Mask.Project(response)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, errSpecTasksListFailed, ErrDetailInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(encoded)
}

func encodeTaskListCursor(codec *pagination.Codec, scope pagination.Scope, cursor *taskusecase.TaskListCursor, sortBy string, asOf time.Time) (string, error) {
	sortValue := cursor.Title
	if sortBy == "due_date" {
		sortValue = cursor.DueDate
	}
	return pagination.Encode(codec, scope, listAnchor{
		At: cursor.CreatedAt, ID: cursor.ID, Direction: cursor.Direction,
		SortValue: sortValue, SortValueNull: cursor.DueDateNull,
		AsOf: asOf.Format(time.RFC3339Nano),
	})
}
