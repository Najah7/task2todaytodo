package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	taskusecase "github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/Najah7/task2todaytodo/internal/port/rest/pagination"
	"github.com/go-chi/chi/v5"
)

var (
	todoItemsListFailure            = NewFailureErrSpec(ResourceTodoItems, ActionList, "Failed to list todo items")
	todoItemsCreateFailure          = ErrSpecTodoItemsCreateFailed
	todoItemsUpdateFailure          = ErrSpecTodoItemsUpdateFailed
	todoItemsDeleteFailure          = ErrSpecTodoItemsDeleteFailed
	todoItemsCompleteFailure        = NewFailureErrSpec(ResourceTodoItems, "complete", "Failed to complete todo item")
	todoItemsReopenFailure          = NewFailureErrSpec(ResourceTodoItems, "reopen", "Failed to reopen todo item")
	todoItemsSkipFailure            = NewFailureErrSpec(ResourceTodoItems, "skip", "Failed to skip todo item")
	todoItemsRestoreFailure         = NewFailureErrSpec(ResourceTodoItems, "restore", "Failed to restore todo item")
	todoItemsReorderFailure         = NewFailureErrSpec(ResourceTodoItems, "reorder", "Failed to reorder todo item")
	todoItemsFrequencyUpdateFailure = NewFailureErrSpec(ResourceTodoItems, "update_frequency", "Failed to update todo item frequency")
)

type TodoItemHandler struct {
	todoItems  taskusecase.TodoItemUseCases
	ID         shared.ID
	pageTokens *pagination.Codec
}

func NewTodoItemHandler(todoItems taskusecase.TodoItemUseCases, ID shared.ID, codecs ...*pagination.Codec) *TodoItemHandler {
	h := &TodoItemHandler{todoItems: todoItems, ID: ID}
	if len(codecs) > 0 {
		h.pageTokens = codecs[0]
	}
	return h
}

type TodoItemResponse struct {
	ID                  string   `json:"id"`
	TaskID              string   `json:"task_id"`
	Title               string   `json:"title"`
	Description         string   `json:"description"`
	DueDate             *string  `json:"due_date"`
	Completed           bool     `json:"completed"`
	Position            int      `json:"position"`
	IntervalWeeks       int      `json:"interval_weeks"`
	Frequencies         []string `json:"frequencies"`
	RepeatState         string   `json:"repeat_state"`
	FrequencyAnchorDate *string  `json:"frequency_anchor_date,omitempty"`
	SeriesID            string   `json:"series_id"`
	OccurrenceDate      string   `json:"occurrence_date"`
	Timezone            string   `json:"timezone"`
	IsException         bool     `json:"is_exception"`
	CreatedAt           int64    `json:"created_at"`
	UpdatedAt           int64    `json:"updated_at"`
}

type TodoItemListResponse struct {
	Items         []TodoItemResponse `json:"items"`
	NextPageToken string             `json:"next_page_token"`
}

type TodoItemCreateRequest struct {
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	DueDate       *string  `json:"due_date"`
	IntervalWeeks int      `json:"interval_weeks"`
	Frequencies   []string `json:"frequencies"`
}

type TodoItemUpdateRequest struct {
	Scope          string               `json:"scope" binding:"required,oneof=current future"`
	OccurrenceDate string               `json:"occurrence_date"`
	Title          optionalJSON[string] `json:"title" swaggertype:"string"`
	Description    optionalJSON[string] `json:"description" swaggertype:"string"`
	DueDate        optionalJSON[string] `json:"due_date" swaggertype:"string"`
}

type TodoItemReorderRequest struct {
	OccurrenceDate string `json:"occurrence_date"`
	Position       *int   `json:"position"`
}

type TodoItemOccurrenceRequest struct {
	OccurrenceDate string `json:"occurrence_date"`
}

type TodoItemFrequencyUpdateRequest struct {
	IntervalWeeks *int     `json:"interval_weeks" binding:"required"`
	Frequencies   []string `json:"frequencies" binding:"required"`
}

const virtualOccurrenceID = "TASK2TODAYTODO000000000000"

func todoItemResponse(item dao.TodoItem) TodoItemResponse {
	id := item.ID
	if !item.IsException && item.SeriesID != "" && item.ID != item.SeriesID {
		id = virtualOccurrenceID
	}
	response := TodoItemResponse{
		ID:             id,
		TaskID:         item.TaskID,
		Title:          item.Title,
		Description:    item.Description,
		Completed:      item.Completed,
		Position:       item.Position,
		IntervalWeeks:  item.IntervalWeeks,
		RepeatState:    item.RepeatState,
		SeriesID:       item.SeriesID,
		OccurrenceDate: item.OccurrenceDate,
		Timezone:       item.Timezone,
		IsException:    item.IsException,
		CreatedAt:      item.CreatedAt,
		UpdatedAt:      item.UpdatedAt,
		Frequencies:    make([]string, 0, len(item.Frequencies)),
	}
	if item.DueDate != 0 {
		dueDate := time.Unix(item.DueDate, 0).UTC().Format("2006-01-02")
		response.DueDate = &dueDate
	}
	for _, frequency := range item.Frequencies {
		response.Frequencies = append(response.Frequencies, frequency.Value)
	}
	if item.FrequencyAnchorDate != 0 {
		anchor := time.Unix(item.FrequencyAnchorDate, 0).UTC().Format("2006-01-02")
		response.FrequencyAnchorDate = &anchor
	}
	return response
}

func todoItemsUserID(r *http.Request) (domain.UserID, bool) {
	userID, ok := r.Context().Value(UserIDContextKey).(string)
	if !ok || userID == "" {
		return "", false
	}
	return domain.UserID(userID), true
}

func todoItemPathIDs(r *http.Request) (domain.TaskID, domain.TodoItemID) {
	return domain.TaskID(chi.URLParam(r, "taskId")), domain.TodoItemID(chi.URLParam(r, "id"))
}

func writeTodoItemError(w http.ResponseWriter, spec ErrSpec, err error) {
	status, detail := todoItemErrorResponse(err)
	WriteError(w, status, spec, detail)
}

func todoItemErrorResponse(err error) (int, ErrDetail) {
	switch {
	case errors.Is(err, taskusecase.ErrPermissionDenied):
		return http.StatusForbidden, NewErrDetail("", "permission_denied", "The caller lacks permission for this todo item operation")
	case errors.Is(err, taskusecase.ErrTaskNotFound), errors.Is(err, taskusecase.ErrTodoItemNotFound):
		return http.StatusNotFound, NewErrDetail("id", "todo_item_not_found", "Task or todo item was not found")
	case errors.Is(err, taskusecase.ErrOccurrenceNotFound):
		return http.StatusNotFound, NewErrDetail("occurrence_date", "occurrence_not_found", "Todo item occurrence was not found")
	case errors.Is(err, taskusecase.ErrOccurrenceDateRequired), errors.Is(err, taskusecase.ErrOccurrenceRuleMismatch):
		return http.StatusBadRequest, NewErrDetail("occurrence_date", "invalid_occurrence", "A valid occurrence date is required")
	case errors.Is(err, taskusecase.ErrOccurrenceInactive):
		return http.StatusConflict, NewErrDetail("occurrence_date", "occurrence_inactive", "Occurrence is no longer active")
	case errors.Is(err, taskusecase.ErrOccurrenceCompleted):
		return http.StatusConflict, NewErrDetail("occurrence_date", "occurrence_completed", "Completed occurrence cannot be skipped")
	case errors.Is(err, taskusecase.ErrTodoItemPositionConflict):
		return http.StatusConflict, NewErrDetail("position", "position_conflict", "Another todo item already uses this position")
	case errors.Is(err, taskusecase.ErrTodoItemPositionOutOfRange):
		return http.StatusBadRequest, NewErrDetail("position", "position_out_of_range", "Position is outside the task's item range")
	case errors.Is(err, taskusecase.ErrTodoItemScopeInvalid):
		return http.StatusBadRequest, NewErrDetail("scope", "invalid_scope", "Scope must be current or future")
	case errors.Is(err, taskusecase.ErrTodoItemFutureDueDateUnsupported):
		return http.StatusBadRequest, NewErrDetail("due_date", "future_due_date_unsupported", "Due date cannot be changed for future occurrences; change recurrence weekdays instead")
	case errors.Is(err, taskusecase.ErrTodoItemPatchRequiredFieldNull):
		return http.StatusBadRequest, NewErrDetail("title", "title_required", "Title cannot be null")
	case errors.Is(err, domain.ErrTodoItemTitleEmpty):
		return http.StatusBadRequest, NewErrDetail("title", "title_required", "Title is required")
	case errors.Is(err, domain.ErrTodoItemPositionLess):
		return http.StatusBadRequest, NewErrDetail("position", "invalid_position", "Position must be zero or greater")
	case errors.Is(err, domain.ErrTodoItemIntervalWeeksLess), errors.Is(err, domain.ErrRecurrenceIntervalWeeksLess):
		return http.StatusBadRequest, NewErrDetail("interval_weeks", "invalid_interval", "Interval weeks must be zero or greater")
	case errors.Is(err, domain.ErrTodoItemIDEmpty):
		return http.StatusInternalServerError, ErrDetailInternalServerError
	case errors.Is(err, domain.ErrTaskFrequencyEmpty), errors.Is(err, domain.ErrTaskFrequencyInvalid):
		return http.StatusBadRequest, NewErrDetail("frequencies", "invalid_frequency", "Frequencies must contain weekday codes")
	case errors.Is(err, domain.ErrRecurrenceTimezoneInvalid), errors.Is(err, domain.ErrRecurrenceMetadataInvalid):
		return http.StatusBadRequest, NewErrDetail("timezone", "invalid_recurrence", "Recurrence settings are invalid")
	default:
		return http.StatusInternalServerError, ErrDetailInternalServerError
	}
}

func todoItemUnauthorized(w http.ResponseWriter, spec ErrSpec) {
	WriteError(w, http.StatusUnauthorized, spec, ErrDetailUnauthorized)
}

func todoItemIDsRequired(w http.ResponseWriter, spec ErrSpec, taskID domain.TaskID, itemID domain.TodoItemID, requireItem bool) bool {
	if taskID == "" || requireItem && itemID == "" {
		WriteError(w, http.StatusBadRequest, spec, NewErrDetail("id", "required", "Task ID and todo item ID are required"))
		return false
	}
	return true
}

func todoItemDate(value *string, field string) (time.Time, error) {
	if value == nil {
		return time.Time{}, nil
	}
	date, err := parseDate(*value)
	if err != nil {
		return time.Time{}, errors.New(field + " must use YYYY-MM-DD format")
	}
	return date, nil
}

func decodeTodoItemOccurrenceDate(r *http.Request) (string, error) {
	if r.Body == nil {
		return "", nil
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, (1<<20)+1))
	if err != nil {
		return "", err
	}
	if len(body) > 1<<20 {
		return "", errors.New("request body too large")
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return "", nil
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(body, &object); err != nil || object == nil {
		return "", errors.New("request body must be a JSON object")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var request TodoItemOccurrenceRequest
	if err := decoder.Decode(&request); err != nil {
		return "", err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return "", errors.New("request body must contain one JSON value")
	}
	return request.OccurrenceDate, nil
}

// List returns all TodoItems in position order for an owned task.
//
//	@Summary		List todo items
//	@Description	Returns TodoItems for the authenticated user's task.
//	@Tags			TodoItems
//	@Produce		json
//	@Security		BearerAuth
//	@Param			taskId		path		string	true	"Task ID"
//	@Param			page_size	query		int		false	"Items per page (default 50, maximum 100)"
//	@Param			page_token	query		string	false	"Opaque next page token"
//	@Param			from_date	query		string	false	"First occurrence date (YYYY-MM-DD); defaults to today in each series timezone"
//	@Param			fields		query		string	false	"Response field mask"
//	@Success		200			{object}	TodoItemListResponse
//	@Failure		400			{object}	ErrResponse	"Invalid task ID"
//	@Failure		401			{object}	ErrResponse	"Unauthorized"
//	@Failure		404			{object}	ErrResponse	"Task not found"
//	@Failure		500			{object}	ErrResponse	"Failed to list todo items"
//
//	@Router			/tasks/{taskId}/todo-items [get]
func (h *TodoItemHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := todoItemsUserID(r)
	if !ok {
		todoItemUnauthorized(w, todoItemsListFailure)
		return
	}
	taskID, _ := todoItemPathIDs(r)
	if !todoItemIDsRequired(w, todoItemsListFailure, taskID, "", false) {
		return
	}
	request, err := parseListRequestWithFromDate(r, h.pageTokens, string(userID), "task_todo_items", string(taskID), "occurrence_date_asc_position_asc_series_id_asc", listEnvelope[TodoItemResponse]{}, true)
	if err != nil {
		writeListError(w, todoItemsListFailure, err)
		return
	}
	var cursor *taskusecase.CursorAnchor
	if request.Anchor != nil {
		cursor = &taskusecase.CursorAnchor{Position: request.Anchor.Position, Date: request.Anchor.OccurrenceDate, SeriesID: request.Anchor.SeriesID, AsOf: request.Anchor.AsOf}
	}
	page, err := h.todoItems.List.ExecutePage(r.Context(), userID, taskID, taskusecase.CursorPageRequest{Size: request.Size, FromDate: request.FromDate, AsOf: request.AsOf, Anchor: cursor})
	if err != nil {
		writeTodoItemError(w, todoItemsListFailure, err)
		return
	}
	items := make([]TodoItemResponse, 0, len(page.Items))
	anchors := make([]listAnchor, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, todoItemResponse(item))
		asOf := ""
		if page.Next != nil {
			asOf = page.Next.AsOf
		}
		anchors = append(anchors, listAnchor{Position: item.Position, OccurrenceDate: item.OccurrenceDate, SeriesID: item.SeriesID, AsOf: asOf})
	}
	writeListResponse(w, items, anchors, page.Next != nil, request, h.pageTokens, todoItemsListFailure)
}

// Create adds a TodoItem to a task. A zero interval creates a one-off item.
//
//	@Summary		Create todo item
//	@Description	Creates a one-off or recurring TodoItem owned by the authenticated user.
//	@Tags			TodoItems
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			taskId	path		string					true	"Task ID"
//	@Param			request	body		TodoItemCreateRequest	true	"TodoItem fields"
//	@Success		201		{object}	TodoItemResponse
//	@Failure		400		{object}	ErrResponse	"Invalid request"
//	@Failure		401		{object}	ErrResponse	"Unauthorized"
//	@Failure		404		{object}	ErrResponse	"Task not found"
//	@Failure		500		{object}	ErrResponse	"Failed to create todo item"
//
//	@Router			/tasks/{taskId}/todo-items [post]
func (h *TodoItemHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := todoItemsUserID(r)
	if !ok {
		todoItemUnauthorized(w, todoItemsCreateFailure)
		return
	}
	taskID, _ := todoItemPathIDs(r)
	if !todoItemIDsRequired(w, todoItemsCreateFailure, taskID, "", false) {
		return
	}
	var request TodoItemCreateRequest
	if err := decodeRequest(r, &request); err != nil {
		WriteError(w, http.StatusBadRequest, todoItemsCreateFailure, ErrDetailInvalidRequestBody)
		return
	}
	dueDate, err := todoItemDate(request.DueDate, "due_date")
	if err != nil {
		WriteError(w, http.StatusBadRequest, todoItemsCreateFailure, NewErrDetail("due_date", "invalid_date", err.Error()))
		return
	}
	item, err := h.todoItems.Create.Execute(r.Context(), taskusecase.CreateTodoItemInput{
		ID: domain.TodoItemID(h.ID.Generate()), UserID: userID, TaskID: taskID,
		Title: request.Title, Description: request.Description, DueDate: dueDate,
		IntervalWeeks: request.IntervalWeeks, Frequencies: request.Frequencies,
	})
	if err != nil {
		writeTodoItemError(w, todoItemsCreateFailure, err)
		return
	}
	WriteJSON(w, http.StatusCreated, todoItemResponse(item))
}

// Update changes one occurrence or the root template using the required body scope.
//
//	@Summary		Update todo item
//	@Description	Updates one occurrence with current scope and occurrence_date. Future scope updates the root template immediately and does not require occurrence_date. due_date cannot be changed with future scope; use the frequency endpoint to change recurrence weekdays. occurrence_date is stable occurrence identity and may differ from due_date after an edit.
//	@Tags			TodoItems
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			taskId	path		string					true	"Task ID"
//	@Param			id		path		string					true	"TodoItem series ID"
//	@Param			request	body		TodoItemUpdateRequest	true	"Fields to update"
//	@Success		200		{object}	TodoItemResponse
//	@Failure		400		{object}	ErrResponse	"Invalid request or scope"
//	@Failure		401		{object}	ErrResponse	"Unauthorized"
//	@Failure		404		{object}	ErrResponse	"Task or todo item not found"
//	@Failure		409		{object}	ErrResponse	"Position conflict"
//	@Failure		500		{object}	ErrResponse	"Failed to update todo item"
//
//	@Router			/tasks/{taskId}/todo-items/{id} [patch]
func (h *TodoItemHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := todoItemsUserID(r)
	if !ok {
		todoItemUnauthorized(w, todoItemsUpdateFailure)
		return
	}
	taskID, itemID := todoItemPathIDs(r)
	if !todoItemIDsRequired(w, todoItemsUpdateFailure, taskID, itemID, true) {
		return
	}
	var request TodoItemUpdateRequest
	if err := decodeRequest(r, &request); err != nil {
		WriteError(w, http.StatusBadRequest, todoItemsUpdateFailure, ErrDetailInvalidRequestBody)
		return
	}
	scope := request.Scope
	if scope != "current" && scope != "future" {
		WriteError(w, http.StatusBadRequest, todoItemsUpdateFailure, NewErrDetail("scope", "invalid_scope", "Scope must be current or future"))
		return
	}
	occurrenceDate := request.OccurrenceDate
	if occurrenceDate != "" {
		parsed, err := parseDate(occurrenceDate)
		if err != nil {
			WriteError(w, http.StatusBadRequest, todoItemsUpdateFailure, NewErrDetail("occurrence_date", "invalid_date", "Occurrence date must use YYYY-MM-DD format"))
			return
		}
		occurrenceDate = parsed.Format("2006-01-02")
	}
	var dueDate taskusecase.PatchField[time.Time]
	if request.DueDate.Present {
		dueDate.Present = true
		if request.DueDate.Value != nil {
			parsed, err := parseDate(*request.DueDate.Value)
			if err != nil {
				WriteError(w, http.StatusBadRequest, todoItemsUpdateFailure, NewErrDetail("due_date", "invalid_date", "Due date must use YYYY-MM-DD format"))
				return
			}
			dueDate.Value = &parsed
		}
	}
	title, description := request.Title.patchField(), request.Description.patchField()
	var item dao.TodoItem
	var err error
	if scope == "future" {
		item, err = h.todoItems.Update.Execute(r.Context(), userID, taskID, itemID, scope, title, description, dueDate)
	} else {
		item, err = h.todoItems.Update.ExecuteOccurrence(r.Context(), userID, taskID, itemID, occurrenceDate, scope, title, description, dueDate)
	}
	if err != nil {
		writeTodoItemError(w, todoItemsUpdateFailure, err)
		return
	}
	WriteJSON(w, http.StatusOK, todoItemResponse(item))
}

// Delete removes an entire TodoItem series or one-off item.
//
//	@Summary		Delete todo item
//	@Description	Deletes the entire TodoItem series, or one-off TodoItem.
//	@Tags			TodoItems
//	@Security		BearerAuth
//	@Param			taskId	path	string	true	"Task ID"
//	@Param			id		path	string	true	"TodoItem series ID"
//	@Success		204
//	@Failure		400	{object}	ErrResponse	"Invalid IDs"
//	@Failure		401	{object}	ErrResponse	"Unauthorized"
//	@Failure		404	{object}	ErrResponse	"Task or todo item not found"
//	@Failure		500	{object}	ErrResponse	"Failed to delete todo item"
//
//	@Router			/tasks/{taskId}/todo-items/{id} [delete]
func (h *TodoItemHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := todoItemsUserID(r)
	if !ok {
		todoItemUnauthorized(w, todoItemsDeleteFailure)
		return
	}
	taskID, itemID := todoItemPathIDs(r)
	if !todoItemIDsRequired(w, todoItemsDeleteFailure, taskID, itemID, true) {
		return
	}
	if err := h.todoItems.Delete.DeleteSeries(r.Context(), userID, taskID, itemID); err != nil {
		writeTodoItemError(w, todoItemsDeleteFailure, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *TodoItemHandler) changeCompleted(w http.ResponseWriter, r *http.Request, spec ErrSpec, execute func(context.Context, domain.UserID, domain.TaskID, domain.TodoItemID, string) error, message string) {
	userID, ok := todoItemsUserID(r)
	if !ok {
		todoItemUnauthorized(w, spec)
		return
	}
	taskID, itemID := todoItemPathIDs(r)
	if !todoItemIDsRequired(w, spec, taskID, itemID, true) {
		return
	}
	occurrenceDate, err := decodeTodoItemOccurrenceDate(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, spec, ErrDetailInvalidRequestBody)
		return
	}
	if occurrenceDate != "" {
		if _, err := parseDate(occurrenceDate); err != nil {
			WriteError(w, http.StatusBadRequest, spec, NewErrDetail("occurrence_date", "invalid_date", "Occurrence date must use YYYY-MM-DD format"))
			return
		}
	}
	if err := execute(r.Context(), userID, taskID, itemID, occurrenceDate); err != nil {
		writeTodoItemError(w, spec, err)
		return
	}
	WriteMessage(w, http.StatusOK, message)
}

// Complete marks one occurrence complete without stopping its series.
//
//	@Summary		Complete todo item
//	@Description	Marks one TodoItem occurrence complete. Recurring items require occurrence_date; one-off items may omit it. occurrence_date identifies scheduled occurrence and may differ from due_date after an edit.
//	@Tags			TodoItems
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			taskId	path		string						true	"Task ID"
//	@Param			id		path		string						true	"TodoItem series ID"
//	@Param			request	body		TodoItemOccurrenceRequest	false	"Stable occurrence date; required for recurring items"
//	@Success		200		{object}	MessageResponse
//	@Failure		400		{object}	ErrResponse
//	@Failure		401		{object}	ErrResponse
//	@Failure		404		{object}	ErrResponse
//	@Failure		500		{object}	ErrResponse
//
//	@Router			/tasks/{taskId}/todo-items/{id}:complete [post]
func (h *TodoItemHandler) Complete(w http.ResponseWriter, r *http.Request) {
	h.changeCompleted(w, r, todoItemsCompleteFailure, h.todoItems.Complete.ExecuteOccurrence, "TodoItem completed")
}

// Reopen clears completion for one occurrence.
//
//	@Summary		Reopen todo item
//	@Description	Reopens one TodoItem occurrence. Recurring items require occurrence_date; one-off items may omit it. occurrence_date identifies scheduled occurrence and may differ from due_date after an edit.
//	@Tags			TodoItems
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			taskId	path		string						true	"Task ID"
//	@Param			id		path		string						true	"TodoItem series ID"
//	@Param			request	body		TodoItemOccurrenceRequest	false	"Stable occurrence date; required for recurring items"
//	@Success		200		{object}	MessageResponse
//	@Failure		400		{object}	ErrResponse
//	@Failure		401		{object}	ErrResponse
//	@Failure		404		{object}	ErrResponse
//	@Failure		500		{object}	ErrResponse
//
//	@Router			/tasks/{taskId}/todo-items/{id}:reopen [post]
func (h *TodoItemHandler) Reopen(w http.ResponseWriter, r *http.Request) {
	h.changeCompleted(w, r, todoItemsReopenFailure, h.todoItems.Reopen.ExecuteOccurrence, "TodoItem reopened")
}

// Skip marks one recurring TodoItem occurrence skipped. occurrence_date identifies stable occurrence, even if due_date changed.
//
//	@Summary	Skip todo item occurrence
//	@Tags		TodoItems
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		taskId	path		string						true	"Task ID"
//	@Param		id		path		string						true	"TodoItem series ID"
//	@Param		request	body		TodoItemOccurrenceRequest	true	"Stable recurring occurrence date"
//	@Success	200		{object}	MessageResponse
//	@Failure	400		{object}	ErrResponse
//	@Failure	401		{object}	ErrResponse
//	@Failure	404		{object}	ErrResponse
//	@Failure	409		{object}	ErrResponse	"Completed occurrence cannot be skipped"
//	@Failure	500		{object}	ErrResponse
//	@Router		/tasks/{taskId}/todo-items/{id}:skip [post]
func (h *TodoItemHandler) Skip(w http.ResponseWriter, r *http.Request) {
	h.changeCompleted(w, r, todoItemsSkipFailure, h.todoItems.Skip.Execute, "TodoItem skipped")
}

// Restore restores one skipped TodoItem occurrence. occurrence_date identifies stable occurrence, even if due_date changed.
//
//	@Summary	Restore todo item occurrence
//	@Tags		TodoItems
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		taskId	path		string						true	"Task ID"
//	@Param		id		path		string						true	"TodoItem series ID"
//	@Param		request	body		TodoItemOccurrenceRequest	true	"Stable recurring occurrence date"
//	@Success	200		{object}	MessageResponse
//	@Failure	400		{object}	ErrResponse
//	@Failure	401		{object}	ErrResponse
//	@Failure	404		{object}	ErrResponse
//	@Failure	500		{object}	ErrResponse
//	@Router		/tasks/{taskId}/todo-items/{id}:restore [post]
func (h *TodoItemHandler) Restore(w http.ResponseWriter, r *http.Request) {
	h.changeCompleted(w, r, todoItemsRestoreFailure, h.todoItems.Restore.Execute, "TodoItem restored")
}

// Reorder moves an occurrence to a zero-based position among that date's items.
//
//	@Summary		Reorder todo item
//	@Description	Moves the TodoItem to a new zero-based position for the same occurrence date.
//	@Tags			TodoItems
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			taskId	path		string					true	"Task ID"
//	@Param			id		path		string					true	"TodoItem series ID"
//	@Param			request	body		TodoItemReorderRequest	true	"Target position"
//	@Success		200		{object}	TodoItemResponse
//	@Failure		400		{object}	ErrResponse	"Invalid position"
//	@Failure		401		{object}	ErrResponse	"Unauthorized"
//	@Failure		404		{object}	ErrResponse	"Task or todo item not found"
//	@Failure		409		{object}	ErrResponse	"Concurrent position conflict"
//	@Failure		500		{object}	ErrResponse	"Failed to reorder todo item"
//
//	@Router			/tasks/{taskId}/todo-items/{id}:reorder [post]
func (h *TodoItemHandler) Reorder(w http.ResponseWriter, r *http.Request) {
	userID, ok := todoItemsUserID(r)
	if !ok {
		todoItemUnauthorized(w, todoItemsReorderFailure)
		return
	}
	taskID, itemID := todoItemPathIDs(r)
	if !todoItemIDsRequired(w, todoItemsReorderFailure, taskID, itemID, true) {
		return
	}
	var request TodoItemReorderRequest
	if err := decodeRequest(r, &request); err != nil || request.Position == nil {
		WriteError(w, http.StatusBadRequest, todoItemsReorderFailure, NewErrDetail("position", "required", "Position is required"))
		return
	}
	if request.OccurrenceDate != "" {
		if _, err := parseDate(request.OccurrenceDate); err != nil {
			WriteError(w, http.StatusBadRequest, todoItemsReorderFailure, NewErrDetail("occurrence_date", "invalid_date", "Occurrence date must use YYYY-MM-DD format"))
			return
		}
	}
	item, err := h.todoItems.Reorder.ExecuteOccurrence(r.Context(), userID, taskID, itemID, request.OccurrenceDate, *request.Position)
	if err != nil {
		writeTodoItemError(w, todoItemsReorderFailure, err)
		return
	}
	WriteJSON(w, http.StatusOK, todoItemResponse(item))
}

// UpdateFrequency changes or stops a root item's recurrence.
//
//	@Summary		Update todo item frequency
//	@Description	Changes recurrence settings immediately on the root item. frequencies selects weekdays and interval_weeks sets the number of weeks between matching weeks. Use this endpoint to change recurrence weekdays. interval_weeks zero stops future generation; occurrence_date is not part of this request.
//	@Tags			TodoItems
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			taskId	path		string							true	"Task ID"
//	@Param			id		path		string							true	"TodoItem root ID"
//	@Param			request	body		TodoItemFrequencyUpdateRequest	true	"Frequency settings"
//	@Success		200		{object}	MessageResponse
//	@Failure		400		{object}	ErrResponse	"Invalid frequency settings"
//	@Failure		401		{object}	ErrResponse	"Unauthorized"
//	@Failure		404		{object}	ErrResponse	"Task or series root not found"
//	@Failure		409		{object}	ErrResponse	"Position conflict"
//	@Failure		500		{object}	ErrResponse	"Failed to update todo item frequency"
//
//	@Router			/tasks/{taskId}/todo-items/{id}/frequency [put]
func (h *TodoItemHandler) UpdateFrequency(w http.ResponseWriter, r *http.Request) {
	userID, ok := todoItemsUserID(r)
	if !ok {
		todoItemUnauthorized(w, todoItemsFrequencyUpdateFailure)
		return
	}
	taskID, itemID := todoItemPathIDs(r)
	if !todoItemIDsRequired(w, todoItemsFrequencyUpdateFailure, taskID, itemID, true) {
		return
	}
	var request TodoItemFrequencyUpdateRequest
	if err := decodeRequest(r, &request); err != nil || request.IntervalWeeks == nil {
		WriteError(w, http.StatusBadRequest, todoItemsFrequencyUpdateFailure, NewErrDetail("interval_weeks", "required", "Interval weeks is required"))
		return
	}
	if *request.IntervalWeeks == domain.OnceIntervalWeeks && len(request.Frequencies) != 0 {
		WriteError(w, http.StatusBadRequest, todoItemsFrequencyUpdateFailure, NewErrDetail("frequencies", "invalid_frequency", "Stopped recurrence cannot include weekday frequencies"))
		return
	}
	err := h.todoItems.UpdateFrequency.Execute(r.Context(), taskusecase.UpdateTodoItemFrequencyInput{
		UserID: userID, TaskID: taskID, TodoItemID: itemID,
		IntervalWeeks: *request.IntervalWeeks, Frequencies: request.Frequencies,
	})
	if err != nil {
		writeTodoItemError(w, todoItemsFrequencyUpdateFailure, err)
		return
	}
	WriteMessage(w, http.StatusOK, "TodoItem frequency updated")
}
