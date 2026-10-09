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
	actionItemsListFailure            = NewFailureErrSpec(ResourceActionItems, ActionList, "Failed to list action items")
	actionItemsCreateFailure          = ErrSpecActionItemsCreateFailed
	actionItemsUpdateFailure          = ErrSpecActionItemsUpdateFailed
	actionItemsDeleteFailure          = ErrSpecActionItemsDeleteFailed
	actionItemsCompleteFailure        = NewFailureErrSpec(ResourceActionItems, "complete", "Failed to complete action item")
	actionItemsReopenFailure          = NewFailureErrSpec(ResourceActionItems, "reopen", "Failed to reopen action item")
	actionItemsSkipFailure            = NewFailureErrSpec(ResourceActionItems, "skip", "Failed to skip action item")
	actionItemsRestoreFailure         = NewFailureErrSpec(ResourceActionItems, "restore", "Failed to restore action item")
	actionItemsReorderFailure         = NewFailureErrSpec(ResourceActionItems, "reorder", "Failed to reorder action item")
	actionItemsFrequencyUpdateFailure = NewFailureErrSpec(ResourceActionItems, "update_frequency", "Failed to update action item frequency")
)

type ActionItemHandler struct {
	actionItems taskusecase.ActionItemUseCases
	ID          shared.ID
	pageTokens  *pagination.Codec
}

func NewActionItemHandler(actionItems taskusecase.ActionItemUseCases, ID shared.ID, codecs ...*pagination.Codec) *ActionItemHandler {
	h := &ActionItemHandler{actionItems: actionItems, ID: ID}
	if len(codecs) > 0 {
		h.pageTokens = codecs[0]
	}
	return h
}

type ActionItemResponse struct {
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

type ActionItemListResponse struct {
	Items         []ActionItemResponse `json:"items"`
	NextPageToken string               `json:"next_page_token"`
}

type ActionItemCreateRequest struct {
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	DueDate       *string  `json:"due_date"`
	IntervalWeeks int      `json:"interval_weeks"`
	Frequencies   []string `json:"frequencies"`
}

type ActionItemUpdateRequest struct {
	Scope          string               `json:"scope" binding:"required,oneof=current future"`
	OccurrenceDate string               `json:"occurrence_date"`
	Title          optionalJSON[string] `json:"title" swaggertype:"string"`
	Description    optionalJSON[string] `json:"description" swaggertype:"string"`
	DueDate        optionalJSON[string] `json:"due_date" swaggertype:"string"`
}

type ActionItemReorderRequest struct {
	OccurrenceDate string `json:"occurrence_date"`
	Position       *int   `json:"position"`
}

type ActionItemOccurrenceRequest struct {
	OccurrenceDate string `json:"occurrence_date"`
}

type ActionItemFrequencyUpdateRequest struct {
	IntervalWeeks *int     `json:"interval_weeks" binding:"required"`
	Frequencies   []string `json:"frequencies" binding:"required"`
}

const virtualOccurrenceID = "TASK2TODAYTODO000000000000"

func actionItemResponse(item dao.ActionItem) ActionItemResponse {
	id := item.ID
	if !item.IsException && item.SeriesID != "" && item.ID != item.SeriesID {
		id = virtualOccurrenceID
	}
	response := ActionItemResponse{
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

func actionItemsUserID(r *http.Request) (domain.UserID, bool) {
	userID, ok := r.Context().Value(UserIDContextKey).(string)
	if !ok || userID == "" {
		return "", false
	}
	return domain.UserID(userID), true
}

func actionItemPathIDs(r *http.Request) (domain.TaskID, domain.ActionItemID) {
	return domain.TaskID(chi.URLParam(r, "taskId")), domain.ActionItemID(chi.URLParam(r, "id"))
}

func writeActionItemError(w http.ResponseWriter, spec ErrSpec, err error) {
	status, detail := actionItemErrorResponse(err)
	WriteError(w, status, spec, detail)
}

func actionItemErrorResponse(err error) (int, ErrDetail) {
	switch {
	case errors.Is(err, taskusecase.ErrPermissionDenied):
		return http.StatusForbidden, NewErrDetail("", "permission_denied", "The caller lacks permission for this action item operation")
	case errors.Is(err, taskusecase.ErrTaskNotFound), errors.Is(err, taskusecase.ErrActionItemNotFound):
		return http.StatusNotFound, NewErrDetail("id", "action_item_not_found", "Task or action item was not found")
	case errors.Is(err, taskusecase.ErrOccurrenceNotFound):
		return http.StatusNotFound, NewErrDetail("occurrence_date", "occurrence_not_found", "Action item occurrence was not found")
	case errors.Is(err, taskusecase.ErrOccurrenceDateRequired), errors.Is(err, taskusecase.ErrOccurrenceRuleMismatch):
		return http.StatusBadRequest, NewErrDetail("occurrence_date", "invalid_occurrence", "A valid occurrence date is required")
	case errors.Is(err, taskusecase.ErrOccurrenceInactive):
		return http.StatusConflict, NewErrDetail("occurrence_date", "occurrence_inactive", "Occurrence is no longer active")
	case errors.Is(err, taskusecase.ErrOccurrenceCompleted):
		return http.StatusConflict, NewErrDetail("occurrence_date", "occurrence_completed", "Completed occurrence cannot be skipped")
	case errors.Is(err, taskusecase.ErrActionItemPositionConflict):
		return http.StatusConflict, NewErrDetail("position", "position_conflict", "Another action item already uses this position")
	case errors.Is(err, taskusecase.ErrActionItemPositionOutOfRange):
		return http.StatusBadRequest, NewErrDetail("position", "position_out_of_range", "Position is outside the task's item range")
	case errors.Is(err, taskusecase.ErrActionItemScopeInvalid):
		return http.StatusBadRequest, NewErrDetail("scope", "invalid_scope", "Scope must be current or future")
	case errors.Is(err, taskusecase.ErrActionItemFutureDueDateUnsupported):
		return http.StatusBadRequest, NewErrDetail("due_date", "future_due_date_unsupported", "Due date cannot be changed for future occurrences; change recurrence weekdays instead")
	case errors.Is(err, taskusecase.ErrActionItemPatchRequiredFieldNull):
		return http.StatusBadRequest, NewErrDetail("title", "title_required", "Title cannot be null")
	case errors.Is(err, domain.ErrActionItemTitleEmpty):
		return http.StatusBadRequest, NewErrDetail("title", "title_required", "Title is required")
	case errors.Is(err, domain.ErrActionItemPositionLess):
		return http.StatusBadRequest, NewErrDetail("position", "invalid_position", "Position must be zero or greater")
	case errors.Is(err, domain.ErrActionItemIntervalWeeksLess), errors.Is(err, domain.ErrRecurrenceIntervalWeeksLess):
		return http.StatusBadRequest, NewErrDetail("interval_weeks", "invalid_interval", "Interval weeks must be zero or greater")
	case errors.Is(err, domain.ErrActionItemIDEmpty):
		return http.StatusInternalServerError, ErrDetailInternalServerError
	case errors.Is(err, domain.ErrTaskFrequencyEmpty), errors.Is(err, domain.ErrTaskFrequencyInvalid):
		return http.StatusBadRequest, NewErrDetail("frequencies", "invalid_frequency", "Frequencies must contain weekday codes")
	case errors.Is(err, domain.ErrRecurrenceTimezoneInvalid), errors.Is(err, domain.ErrRecurrenceMetadataInvalid):
		return http.StatusBadRequest, NewErrDetail("timezone", "invalid_recurrence", "Recurrence settings are invalid")
	default:
		return http.StatusInternalServerError, ErrDetailInternalServerError
	}
}

func actionItemUnauthorized(w http.ResponseWriter, spec ErrSpec) {
	WriteError(w, http.StatusUnauthorized, spec, ErrDetailUnauthorized)
}

func actionItemIDsRequired(w http.ResponseWriter, spec ErrSpec, taskID domain.TaskID, itemID domain.ActionItemID, requireItem bool) bool {
	if taskID == "" || requireItem && itemID == "" {
		WriteError(w, http.StatusBadRequest, spec, NewErrDetail("id", "required", "Task ID and action item ID are required"))
		return false
	}
	return true
}

func actionItemDate(value *string, field string) (time.Time, error) {
	if value == nil {
		return time.Time{}, nil
	}
	date, err := parseDate(*value)
	if err != nil {
		return time.Time{}, errors.New(field + " must use YYYY-MM-DD format")
	}
	return date, nil
}

func decodeActionItemOccurrenceDate(r *http.Request) (string, error) {
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
	var request ActionItemOccurrenceRequest
	if err := decoder.Decode(&request); err != nil {
		return "", err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return "", errors.New("request body must contain one JSON value")
	}
	return request.OccurrenceDate, nil
}

// List returns all ActionItems in position order for an owned task.
//
//	@Summary		List action items
//	@Description	Returns ActionItems for the authenticated user's task.
//	@Tags			ActionItems
//	@Produce		json
//	@Security		BearerAuth
//	@Param			taskId		path		string	true	"Task ID"
//	@Param			page_size	query		int		false	"Items per page (default 50, maximum 100)"
//	@Param			page_token	query		string	false	"Opaque next page token"
//	@Param			from_date	query		string	false	"First occurrence date (YYYY-MM-DD); defaults to today in each series timezone"
//	@Param			fields		query		string	false	"Response field mask"
//	@Success		200			{object}	ActionItemListResponse
//	@Failure		400			{object}	ErrResponse	"Invalid task ID"
//	@Failure		401			{object}	ErrResponse	"Unauthorized"
//	@Failure		404			{object}	ErrResponse	"Task not found"
//	@Failure		500			{object}	ErrResponse	"Failed to list action items"
//
//	@Router			/tasks/{taskId}/action-items [get]
func (h *ActionItemHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := actionItemsUserID(r)
	if !ok {
		actionItemUnauthorized(w, actionItemsListFailure)
		return
	}
	taskID, _ := actionItemPathIDs(r)
	if !actionItemIDsRequired(w, actionItemsListFailure, taskID, "", false) {
		return
	}
	request, err := parseListRequestWithFromDate(r, h.pageTokens, string(userID), "task_action_items", string(taskID), "occurrence_date_asc_position_asc_series_id_asc", listEnvelope[ActionItemResponse]{}, true)
	if err != nil {
		writeListError(w, actionItemsListFailure, err)
		return
	}
	var cursor *taskusecase.CursorAnchor
	if request.Anchor != nil {
		cursor = &taskusecase.CursorAnchor{Position: request.Anchor.Position, Date: request.Anchor.OccurrenceDate, SeriesID: request.Anchor.SeriesID, AsOf: request.Anchor.AsOf}
	}
	page, err := h.actionItems.List.ExecutePage(r.Context(), userID, taskID, taskusecase.CursorPageRequest{Size: request.Size, FromDate: request.FromDate, AsOf: request.AsOf, Anchor: cursor})
	if err != nil {
		writeActionItemError(w, actionItemsListFailure, err)
		return
	}
	items := make([]ActionItemResponse, 0, len(page.Items))
	anchors := make([]listAnchor, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, actionItemResponse(item))
		asOf := ""
		if page.Next != nil {
			asOf = page.Next.AsOf
		}
		anchors = append(anchors, listAnchor{Position: item.Position, OccurrenceDate: item.OccurrenceDate, SeriesID: item.SeriesID, AsOf: asOf})
	}
	writeListResponse(w, items, anchors, page.Next != nil, request, h.pageTokens, actionItemsListFailure)
}

// Create adds a ActionItem to a task. A zero interval creates a one-off item.
//
//	@Summary		Create action item
//	@Description	Creates a one-off or recurring ActionItem owned by the authenticated user.
//	@Tags			ActionItems
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			taskId	path		string					true	"Task ID"
//	@Param			request	body		ActionItemCreateRequest	true	"ActionItem fields"
//	@Success		201		{object}	ActionItemResponse
//	@Failure		400		{object}	ErrResponse	"Invalid request"
//	@Failure		401		{object}	ErrResponse	"Unauthorized"
//	@Failure		404		{object}	ErrResponse	"Task not found"
//	@Failure		500		{object}	ErrResponse	"Failed to create action item"
//
//	@Router			/tasks/{taskId}/action-items [post]
func (h *ActionItemHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := actionItemsUserID(r)
	if !ok {
		actionItemUnauthorized(w, actionItemsCreateFailure)
		return
	}
	taskID, _ := actionItemPathIDs(r)
	if !actionItemIDsRequired(w, actionItemsCreateFailure, taskID, "", false) {
		return
	}
	var request ActionItemCreateRequest
	if err := decodeRequest(r, &request); err != nil {
		WriteError(w, http.StatusBadRequest, actionItemsCreateFailure, ErrDetailInvalidRequestBody)
		return
	}
	dueDate, err := actionItemDate(request.DueDate, "due_date")
	if err != nil {
		WriteError(w, http.StatusBadRequest, actionItemsCreateFailure, NewErrDetail("due_date", "invalid_date", err.Error()))
		return
	}
	item, err := h.actionItems.Create.Execute(r.Context(), taskusecase.CreateActionItemInput{
		ID: domain.ActionItemID(h.ID.Generate()), UserID: userID, TaskID: taskID,
		Title: request.Title, Description: request.Description, DueDate: dueDate,
		IntervalWeeks: request.IntervalWeeks, Frequencies: request.Frequencies,
	})
	if err != nil {
		writeActionItemError(w, actionItemsCreateFailure, err)
		return
	}
	WriteJSON(w, http.StatusCreated, actionItemResponse(item))
}

// Update changes one occurrence or the root template using the required body scope.
//
//	@Summary		Update action item
//	@Description	Updates one occurrence with current scope and occurrence_date. Future scope updates the root template immediately and does not require occurrence_date. due_date cannot be changed with future scope; use the frequency endpoint to change recurrence weekdays. occurrence_date is stable occurrence identity and may differ from due_date after an edit.
//	@Tags			ActionItems
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			taskId	path		string					true	"Task ID"
//	@Param			id		path		string					true	"ActionItem series ID"
//	@Param			request	body		ActionItemUpdateRequest	true	"Fields to update"
//	@Success		200		{object}	ActionItemResponse
//	@Failure		400		{object}	ErrResponse	"Invalid request or scope"
//	@Failure		401		{object}	ErrResponse	"Unauthorized"
//	@Failure		404		{object}	ErrResponse	"Task or action item not found"
//	@Failure		409		{object}	ErrResponse	"Position conflict"
//	@Failure		500		{object}	ErrResponse	"Failed to update action item"
//
//	@Router			/tasks/{taskId}/action-items/{id} [patch]
func (h *ActionItemHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := actionItemsUserID(r)
	if !ok {
		actionItemUnauthorized(w, actionItemsUpdateFailure)
		return
	}
	taskID, itemID := actionItemPathIDs(r)
	if !actionItemIDsRequired(w, actionItemsUpdateFailure, taskID, itemID, true) {
		return
	}
	var request ActionItemUpdateRequest
	if err := decodeRequest(r, &request); err != nil {
		WriteError(w, http.StatusBadRequest, actionItemsUpdateFailure, ErrDetailInvalidRequestBody)
		return
	}
	scope := request.Scope
	if scope != "current" && scope != "future" {
		WriteError(w, http.StatusBadRequest, actionItemsUpdateFailure, NewErrDetail("scope", "invalid_scope", "Scope must be current or future"))
		return
	}
	occurrenceDate := request.OccurrenceDate
	if occurrenceDate != "" {
		parsed, err := parseDate(occurrenceDate)
		if err != nil {
			WriteError(w, http.StatusBadRequest, actionItemsUpdateFailure, NewErrDetail("occurrence_date", "invalid_date", "Occurrence date must use YYYY-MM-DD format"))
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
				WriteError(w, http.StatusBadRequest, actionItemsUpdateFailure, NewErrDetail("due_date", "invalid_date", "Due date must use YYYY-MM-DD format"))
				return
			}
			dueDate.Value = &parsed
		}
	}
	title, description := request.Title.patchField(), request.Description.patchField()
	var item dao.ActionItem
	var err error
	if scope == "future" {
		item, err = h.actionItems.Update.Execute(r.Context(), userID, taskID, itemID, scope, title, description, dueDate)
	} else {
		item, err = h.actionItems.Update.ExecuteOccurrence(r.Context(), userID, taskID, itemID, occurrenceDate, scope, title, description, dueDate)
	}
	if err != nil {
		writeActionItemError(w, actionItemsUpdateFailure, err)
		return
	}
	WriteJSON(w, http.StatusOK, actionItemResponse(item))
}

// Delete removes an entire ActionItem series or one-off item.
//
//	@Summary		Delete action item
//	@Description	Deletes the entire ActionItem series, or one-off ActionItem.
//	@Tags			ActionItems
//	@Security		BearerAuth
//	@Param			taskId	path	string	true	"Task ID"
//	@Param			id		path	string	true	"ActionItem series ID"
//	@Success		204
//	@Failure		400	{object}	ErrResponse	"Invalid IDs"
//	@Failure		401	{object}	ErrResponse	"Unauthorized"
//	@Failure		404	{object}	ErrResponse	"Task or action item not found"
//	@Failure		500	{object}	ErrResponse	"Failed to delete action item"
//
//	@Router			/tasks/{taskId}/action-items/{id} [delete]
func (h *ActionItemHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := actionItemsUserID(r)
	if !ok {
		actionItemUnauthorized(w, actionItemsDeleteFailure)
		return
	}
	taskID, itemID := actionItemPathIDs(r)
	if !actionItemIDsRequired(w, actionItemsDeleteFailure, taskID, itemID, true) {
		return
	}
	if err := h.actionItems.Delete.DeleteSeries(r.Context(), userID, taskID, itemID); err != nil {
		writeActionItemError(w, actionItemsDeleteFailure, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ActionItemHandler) changeCompleted(w http.ResponseWriter, r *http.Request, spec ErrSpec, execute func(context.Context, domain.UserID, domain.TaskID, domain.ActionItemID, string) error, message string) {
	userID, ok := actionItemsUserID(r)
	if !ok {
		actionItemUnauthorized(w, spec)
		return
	}
	taskID, itemID := actionItemPathIDs(r)
	if !actionItemIDsRequired(w, spec, taskID, itemID, true) {
		return
	}
	occurrenceDate, err := decodeActionItemOccurrenceDate(r)
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
		writeActionItemError(w, spec, err)
		return
	}
	WriteMessage(w, http.StatusOK, message)
}

// Complete marks one occurrence complete without stopping its series.
//
//	@Summary		Complete action item
//	@Description	Marks one ActionItem occurrence complete. Recurring items require occurrence_date; one-off items may omit it. occurrence_date identifies scheduled occurrence and may differ from due_date after an edit.
//	@Tags			ActionItems
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			taskId	path		string						true	"Task ID"
//	@Param			id		path		string						true	"ActionItem series ID"
//	@Param			request	body		ActionItemOccurrenceRequest	false	"Stable occurrence date; required for recurring items"
//	@Success		200		{object}	MessageResponse
//	@Failure		400		{object}	ErrResponse
//	@Failure		401		{object}	ErrResponse
//	@Failure		404		{object}	ErrResponse
//	@Failure		500		{object}	ErrResponse
//
//	@Router			/tasks/{taskId}/action-items/{id}:complete [post]
func (h *ActionItemHandler) Complete(w http.ResponseWriter, r *http.Request) {
	h.changeCompleted(w, r, actionItemsCompleteFailure, h.actionItems.Complete.ExecuteOccurrence, "ActionItem completed")
}

// Reopen clears completion for one occurrence.
//
//	@Summary		Reopen action item
//	@Description	Reopens one ActionItem occurrence. Recurring items require occurrence_date; one-off items may omit it. occurrence_date identifies scheduled occurrence and may differ from due_date after an edit.
//	@Tags			ActionItems
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			taskId	path		string						true	"Task ID"
//	@Param			id		path		string						true	"ActionItem series ID"
//	@Param			request	body		ActionItemOccurrenceRequest	false	"Stable occurrence date; required for recurring items"
//	@Success		200		{object}	MessageResponse
//	@Failure		400		{object}	ErrResponse
//	@Failure		401		{object}	ErrResponse
//	@Failure		404		{object}	ErrResponse
//	@Failure		500		{object}	ErrResponse
//
//	@Router			/tasks/{taskId}/action-items/{id}:reopen [post]
func (h *ActionItemHandler) Reopen(w http.ResponseWriter, r *http.Request) {
	h.changeCompleted(w, r, actionItemsReopenFailure, h.actionItems.Reopen.ExecuteOccurrence, "ActionItem reopened")
}

// Skip marks one recurring ActionItem occurrence skipped. occurrence_date identifies stable occurrence, even if due_date changed.
//
//	@Summary	Skip action item occurrence
//	@Tags		ActionItems
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		taskId	path		string						true	"Task ID"
//	@Param		id		path		string						true	"ActionItem series ID"
//	@Param		request	body		ActionItemOccurrenceRequest	true	"Stable recurring occurrence date"
//	@Success	200		{object}	MessageResponse
//	@Failure	400		{object}	ErrResponse
//	@Failure	401		{object}	ErrResponse
//	@Failure	404		{object}	ErrResponse
//	@Failure	409		{object}	ErrResponse	"Completed occurrence cannot be skipped"
//	@Failure	500		{object}	ErrResponse
//	@Router		/tasks/{taskId}/action-items/{id}:skip [post]
func (h *ActionItemHandler) Skip(w http.ResponseWriter, r *http.Request) {
	h.changeCompleted(w, r, actionItemsSkipFailure, h.actionItems.Skip.Execute, "ActionItem skipped")
}

// Restore restores one skipped ActionItem occurrence. occurrence_date identifies stable occurrence, even if due_date changed.
//
//	@Summary	Restore action item occurrence
//	@Tags		ActionItems
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		taskId	path		string						true	"Task ID"
//	@Param		id		path		string						true	"ActionItem series ID"
//	@Param		request	body		ActionItemOccurrenceRequest	true	"Stable recurring occurrence date"
//	@Success	200		{object}	MessageResponse
//	@Failure	400		{object}	ErrResponse
//	@Failure	401		{object}	ErrResponse
//	@Failure	404		{object}	ErrResponse
//	@Failure	500		{object}	ErrResponse
//	@Router		/tasks/{taskId}/action-items/{id}:restore [post]
func (h *ActionItemHandler) Restore(w http.ResponseWriter, r *http.Request) {
	h.changeCompleted(w, r, actionItemsRestoreFailure, h.actionItems.Restore.Execute, "ActionItem restored")
}

// Reorder moves an occurrence to a zero-based position among that date's items.
//
//	@Summary		Reorder action item
//	@Description	Moves the ActionItem to a new zero-based position for the same occurrence date.
//	@Tags			ActionItems
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			taskId	path		string						true	"Task ID"
//	@Param			id		path		string						true	"ActionItem series ID"
//	@Param			request	body		ActionItemReorderRequest	true	"Target position"
//	@Success		200		{object}	ActionItemResponse
//	@Failure		400		{object}	ErrResponse	"Invalid position"
//	@Failure		401		{object}	ErrResponse	"Unauthorized"
//	@Failure		404		{object}	ErrResponse	"Task or action item not found"
//	@Failure		409		{object}	ErrResponse	"Concurrent position conflict"
//	@Failure		500		{object}	ErrResponse	"Failed to reorder action item"
//
//	@Router			/tasks/{taskId}/action-items/{id}:reorder [post]
func (h *ActionItemHandler) Reorder(w http.ResponseWriter, r *http.Request) {
	userID, ok := actionItemsUserID(r)
	if !ok {
		actionItemUnauthorized(w, actionItemsReorderFailure)
		return
	}
	taskID, itemID := actionItemPathIDs(r)
	if !actionItemIDsRequired(w, actionItemsReorderFailure, taskID, itemID, true) {
		return
	}
	var request ActionItemReorderRequest
	if err := decodeRequest(r, &request); err != nil || request.Position == nil {
		WriteError(w, http.StatusBadRequest, actionItemsReorderFailure, NewErrDetail("position", "required", "Position is required"))
		return
	}
	if request.OccurrenceDate != "" {
		if _, err := parseDate(request.OccurrenceDate); err != nil {
			WriteError(w, http.StatusBadRequest, actionItemsReorderFailure, NewErrDetail("occurrence_date", "invalid_date", "Occurrence date must use YYYY-MM-DD format"))
			return
		}
	}
	item, err := h.actionItems.Reorder.ExecuteOccurrence(r.Context(), userID, taskID, itemID, request.OccurrenceDate, *request.Position)
	if err != nil {
		writeActionItemError(w, actionItemsReorderFailure, err)
		return
	}
	WriteJSON(w, http.StatusOK, actionItemResponse(item))
}

// UpdateFrequency changes or stops a root item's recurrence.
//
//	@Summary		Update action item frequency
//	@Description	Changes recurrence settings immediately on the root item. frequencies selects weekdays and interval_weeks sets the number of weeks between matching weeks. Use this endpoint to change recurrence weekdays. interval_weeks zero stops future generation; occurrence_date is not part of this request.
//	@Tags			ActionItems
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			taskId	path		string								true	"Task ID"
//	@Param			id		path		string								true	"ActionItem root ID"
//	@Param			request	body		ActionItemFrequencyUpdateRequest	true	"Frequency settings"
//	@Success		200		{object}	MessageResponse
//	@Failure		400		{object}	ErrResponse	"Invalid frequency settings"
//	@Failure		401		{object}	ErrResponse	"Unauthorized"
//	@Failure		404		{object}	ErrResponse	"Task or series root not found"
//	@Failure		409		{object}	ErrResponse	"Position conflict"
//	@Failure		500		{object}	ErrResponse	"Failed to update action item frequency"
//
//	@Router			/tasks/{taskId}/action-items/{id}/frequency [put]
func (h *ActionItemHandler) UpdateFrequency(w http.ResponseWriter, r *http.Request) {
	userID, ok := actionItemsUserID(r)
	if !ok {
		actionItemUnauthorized(w, actionItemsFrequencyUpdateFailure)
		return
	}
	taskID, itemID := actionItemPathIDs(r)
	if !actionItemIDsRequired(w, actionItemsFrequencyUpdateFailure, taskID, itemID, true) {
		return
	}
	var request ActionItemFrequencyUpdateRequest
	if err := decodeRequest(r, &request); err != nil || request.IntervalWeeks == nil {
		WriteError(w, http.StatusBadRequest, actionItemsFrequencyUpdateFailure, NewErrDetail("interval_weeks", "required", "Interval weeks is required"))
		return
	}
	if *request.IntervalWeeks == domain.OnceIntervalWeeks && len(request.Frequencies) != 0 {
		WriteError(w, http.StatusBadRequest, actionItemsFrequencyUpdateFailure, NewErrDetail("frequencies", "invalid_frequency", "Stopped recurrence cannot include weekday frequencies"))
		return
	}
	err := h.actionItems.UpdateFrequency.Execute(r.Context(), taskusecase.UpdateActionItemFrequencyInput{
		UserID: userID, TaskID: taskID, ActionItemID: itemID,
		IntervalWeeks: *request.IntervalWeeks, Frequencies: request.Frequencies,
	})
	if err != nil {
		writeActionItemError(w, actionItemsFrequencyUpdateFailure, err)
		return
	}
	WriteMessage(w, http.StatusOK, "ActionItem frequency updated")
}
