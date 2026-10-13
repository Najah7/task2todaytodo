package rest

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	taskusecase "github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/Najah7/task2todaytodo/internal/port/rest/pagination"
	"github.com/go-chi/chi/v5"
)

var (
	actionItemsListFailure     = NewFailureErrSpec(ResourceActionItems, ActionList, "Failed to list action items")
	actionItemsCreateFailure   = ErrSpecActionItemsCreateFailed
	actionItemsUpdateFailure   = ErrSpecActionItemsUpdateFailed
	actionItemsDeleteFailure   = ErrSpecActionItemsDeleteFailed
	actionItemsCompleteFailure = NewFailureErrSpec(ResourceActionItems, "complete", "Failed to complete action item")
	actionItemsReopenFailure   = NewFailureErrSpec(ResourceActionItems, "reopen", "Failed to reopen action item")
	actionItemsReorderFailure  = NewFailureErrSpec(ResourceActionItems, "reorder", "Failed to reorder action item")
)

type ActionItemHandler struct {
	actionItems taskusecase.ActionItemUseCases
	issueID     func() string
	pageTokens  *pagination.Codec
}

func NewActionItemHandler(actionItems taskusecase.ActionItemUseCases, issueID func() string, codecs ...*pagination.Codec) *ActionItemHandler {
	h := &ActionItemHandler{actionItems: actionItems, issueID: issueID}
	if len(codecs) > 0 {
		h.pageTokens = codecs[0]
	}
	return h
}

type ActionItemResponse struct {
	ID               string  `json:"id"`
	TaskID           string  `json:"task_id"`
	Title            string  `json:"title"`
	Description      string  `json:"description"`
	EstimatedMinutes int     `json:"estimated_minutes" validate:"required" minimum:"1" maximum:"2147483647"`
	ActualMinutes    *int    `json:"actual_minutes" extensions:"x-nullable" minimum:"0" maximum:"2147483647"`
	Started          bool    `json:"started"`
	EstimateEditable bool    `json:"estimate_editable"`
	Priority         string  `json:"priority"`
	DueDate          *string `json:"due_date"`
	Completed        bool    `json:"completed"`
	Position         int     `json:"position"`
	CreatedAt        int64   `json:"created_at"`
	UpdatedAt        int64   `json:"updated_at"`
}

type ActionItemListResponse struct {
	Items         []ActionItemResponse `json:"items"`
	NextPageToken string               `json:"next_page_token"`
}

type ActionItemCreateRequest struct {
	Title            string  `json:"title"`
	Description      string  `json:"description"`
	EstimatedMinutes *int    `json:"estimated_minutes" validate:"required" minimum:"1" maximum:"2147483647"`
	Priority         string  `json:"priority"`
	DueDate          *string `json:"due_date"`
}

type ActionItemUpdateRequest struct {
	Title            optionalJSON[string] `json:"title" swaggertype:"string"`
	Description      optionalJSON[string] `json:"description" swaggertype:"string"`
	DueDate          optionalJSON[string] `json:"due_date" swaggertype:"string"`
	EstimatedMinutes optionalJSON[int]    `json:"estimated_minutes" swaggertype:"integer" minimum:"1" maximum:"2147483647"`
	ActualMinutes    optionalJSON[int]    `json:"actual_minutes" swaggertype:"integer" extensions:"x-nullable" minimum:"0" maximum:"2147483647"`
	Priority         optionalJSON[string] `json:"priority" swaggertype:"string"`
}

type ActionItemReorderRequest struct {
	Position *int `json:"position"`
}

type ActionItemCompleteRequest struct {
	ActualMinutes *int `json:"actual_minutes" extensions:"x-nullable" minimum:"0" maximum:"2147483647"`
}

type ActionItemDeleteRequest struct {
	ManualEstimatedMinutes optionalJSON[int] `json:"manual_estimated_minutes" swaggertype:"integer" minimum:"1" maximum:"2147483647"`
}

func actionItemResponse(item dao.ActionItem) ActionItemResponse {
	response := ActionItemResponse{
		ID:               item.ID,
		TaskID:           item.TaskID,
		Title:            item.Title,
		Description:      item.Description,
		EstimatedMinutes: valueInt(item.EstimatedMinutes),
		ActualMinutes:    cloneInt(item.ActualMinutes),
		Started:          item.Started,
		EstimateEditable: item.EstimateEditable,
		Priority:         item.Priority.Value,
		Completed:        item.Completed,
		Position:         item.Position,
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
	}
	if item.DueDate != 0 {
		dueDate := time.Unix(item.DueDate, 0).UTC().Format("2006-01-02")
		response.DueDate = &dueDate
	}
	return response
}

func valueInt(value *int) int {
	if value == nil {
		return 0
	}
	return *value
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
	case errors.Is(err, taskusecase.ErrRevisionConflict):
		return http.StatusPreconditionFailed, NewErrDetail("If-Match", "revision_mismatch", "The Task changed since the supplied revision")
	case errors.Is(err, taskusecase.ErrFreeActionItemLimitReached):
		return http.StatusConflict, NewErrDetail("plan", "free_action_item_limit_reached", "The Free plan allows up to 100 ActionItems in each Project or across projectless Tasks")
	case errors.Is(err, taskusecase.ErrTeamSubscriptionRequired):
		return http.StatusForbidden, NewErrDetail("plan", "team_required", "A Team subscription is required to write shared Project work")
	case errors.Is(err, taskusecase.ErrPermissionDenied):
		return http.StatusForbidden, NewErrDetail("", "permission_denied", "The caller lacks permission for this action item operation")
	case errors.Is(err, taskusecase.ErrTaskNotFound), errors.Is(err, taskusecase.ErrActionItemNotFound):
		return http.StatusNotFound, NewErrDetail("id", "action_item_not_found", "Task or action item was not found")
	case errors.Is(err, taskusecase.ErrActionItemCompleted):
		return http.StatusConflict, NewErrDetail("id", "action_item_completed", "Completed action item cannot be deleted")
	case errors.Is(err, taskusecase.ErrActionItemPositionOutOfRange):
		return http.StatusBadRequest, NewErrDetail("position", "position_out_of_range", "Position is outside the task's item range")
	case errors.Is(err, taskusecase.ErrActionItemPatchRequiredFieldNull):
		return http.StatusBadRequest, NewErrDetail("title", "title_required", "Title cannot be null")
	case errors.Is(err, domain.ErrActionItemEstimatedMinutesInvalid):
		return http.StatusBadRequest, NewErrDetail("estimated_minutes", "invalid_minutes", "Estimated minutes must be between 1 and 2147483647")
	case errors.Is(err, domain.ErrActionItemActualMinutesInvalid):
		return http.StatusBadRequest, NewErrDetail("actual_minutes", "invalid_minutes", "Actual minutes must be between 0 and 2147483647")
	case errors.Is(err, taskusecase.ErrActionItemActualMinutesRequiresCompletion):
		return http.StatusConflict, NewErrDetail("actual_minutes", "action_item_not_completed", "Actual minutes can only be edited after completion")
	case errors.Is(err, taskusecase.ErrActionItemStarted):
		return http.StatusConflict, NewErrDetail("estimated_minutes", "action_item_started", "Estimate cannot change after an ActionItem has started")
	case errors.Is(err, taskusecase.ErrTaskManualEstimateRequired):
		return http.StatusBadRequest, NewErrDetail("manual_estimated_minutes", "required", "A positive estimate is required when a Task has no ActionItems")
	case errors.Is(err, taskusecase.ErrTaskStarted):
		return http.StatusConflict, NewErrDetail("manual_estimated_minutes", "task_started", "Task estimate cannot change after work starts or completes")
	case errors.Is(err, domain.ErrTaskEstimatedMinutesInvalid):
		return http.StatusBadRequest, NewErrDetail("manual_estimated_minutes", "invalid_minutes", "Estimated minutes must be between 1 and 2147483647")
	case errors.Is(err, taskusecase.ErrCannotAddActionItemToStartedTask):
		return http.StatusConflict, NewErrDetail("task_id", "task_started", "ActionItems cannot be added to a started or completed childless Task")
	case errors.Is(err, domain.ErrTaskPriorityEmpty), errors.Is(err, domain.ErrTaskPriorityInvalid):
		return http.StatusBadRequest, NewErrDetail("priority", "invalid_priority", "Priority is invalid")
	case errors.Is(err, domain.ErrActionItemTitleEmpty):
		return http.StatusBadRequest, NewErrDetail("title", "title_required", "Title is required")
	case errors.Is(err, domain.ErrActionItemPositionLess):
		return http.StatusBadRequest, NewErrDetail("position", "invalid_position", "Position must be zero or greater")
	case errors.Is(err, domain.ErrActionItemIDEmpty):
		return http.StatusInternalServerError, ErrDetailInternalServerError
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
	request, err := parseListRequest(r, h.pageTokens, string(userID), "task_action_items", string(taskID), "position_asc_id_asc", listEnvelope[ActionItemResponse]{})
	if err != nil {
		writeListError(w, actionItemsListFailure, err)
		return
	}
	var cursor *taskusecase.CursorAnchor
	if request.Anchor != nil {
		cursor = &taskusecase.CursorAnchor{Position: request.Anchor.Position, ID: request.Anchor.ID}
	}
	page, err := h.actionItems.List.ExecutePage(r.Context(), userID, taskID, taskusecase.CursorPageRequest{Size: request.Size, Anchor: cursor})
	if err != nil {
		writeActionItemError(w, actionItemsListFailure, err)
		return
	}
	items := make([]ActionItemResponse, 0, len(page.Items))
	anchors := make([]listAnchor, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, actionItemResponse(item))
		anchors = append(anchors, listAnchor{Position: item.Position, ID: item.ID})
	}
	writeListResponse(w, items, anchors, page.Next != nil, request, h.pageTokens, actionItemsListFailure)
}

// Create adds a single ActionItem to a task.
//
//	@Summary		Create action item
//	@Description	Creates a single ActionItem owned by the authenticated user.
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
		ID: domain.ActionItemID(h.issueID()), UserID: userID, TaskID: taskID,
		Title: request.Title, Description: request.Description, DueDate: dueDate,
		EstimatedMinutes: cloneInt(request.EstimatedMinutes), Priority: request.Priority,
	})
	if err != nil {
		writeActionItemError(w, actionItemsCreateFailure, err)
		return
	}
	WriteJSON(w, http.StatusCreated, actionItemResponse(item))
}

// Update changes a saved ActionItem.
//
//	@Summary		Update action item
//	@Description	Updates a saved ActionItem.
//	@Tags			ActionItems
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			taskId	path		string					true	"Task ID"
//	@Param			id		path		string					true	"ActionItem ID"
//	@Param			request	body		ActionItemUpdateRequest	true	"Fields to update"
//	@Success		200		{object}	ActionItemResponse
//	@Failure		400		{object}	ErrResponse	"Invalid request"
//	@Failure		401		{object}	ErrResponse	"Unauthorized"
//	@Failure		404		{object}	ErrResponse	"Task or action item not found"
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
	planning := taskusecase.ActionItemPlanningPatch{
		EstimatedMinutes: request.EstimatedMinutes.patchField(), ActualMinutes: request.ActualMinutes.patchField(),
		Priority: request.Priority.patchField(),
	}
	item, err = h.actionItems.Update.Execute(r.Context(), userID, taskID, itemID, title, description, dueDate, planning)
	if err != nil {
		writeActionItemError(w, actionItemsUpdateFailure, err)
		return
	}
	WriteJSON(w, http.StatusOK, actionItemResponse(item))
}

// Delete removes a saved ActionItem.
//
//	@Summary		Delete action item
//	@Description	Deletes a saved ActionItem. When removing the final child of a Task, an optional body atomically supplies the childless Task's manual estimate.
//	@Tags			ActionItems
//	@Accept			json
//	@Security		BearerAuth
//	@Param			taskId		path	string					true	"Task ID"
//	@Param			id			path	string					true	"ActionItem ID"
//	@Param			If-Match	header	string					false	"Current Task ETag; required when manual_estimated_minutes is supplied"
//	@Param			request		body	ActionItemDeleteRequest	false	"Optional last-child Task estimate"
//	@Success		204
//	@Failure		400	{object}	ErrResponse	"Invalid IDs"
//	@Failure		401	{object}	ErrResponse	"Unauthorized"
//	@Failure		404	{object}	ErrResponse	"Task or action item not found"
//	@Failure		409	{object}	ErrResponse	"Completed ActionItems cannot be deleted"
//	@Failure		412	{object}	ErrResponse	"Task revision does not match If-Match"
//	@Failure		428	{object}	ErrResponse	"If-Match is required when changing the Task manual estimate"
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
	var request ActionItemDeleteRequest
	if r.Body != nil {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			WriteError(w, http.StatusBadRequest, actionItemsDeleteFailure, ErrDetailInvalidRequestBody)
			return
		}
		if len(bytes.TrimSpace(body)) > 0 {
			r.Body = io.NopCloser(bytes.NewReader(body))
			if err := decodeRequest(r, &request); err != nil {
				WriteError(w, http.StatusBadRequest, actionItemsDeleteFailure, ErrDetailInvalidRequestBody)
				return
			}
		}
	}
	var expected *int32
	if request.ManualEstimatedMinutes.Present {
		if request.ManualEstimatedMinutes.Value == nil || *request.ManualEstimatedMinutes.Value < 1 || int64(*request.ManualEstimatedMinutes.Value) > 2147483647 {
			WriteError(w, http.StatusBadRequest, actionItemsDeleteFailure, NewErrDetail("manual_estimated_minutes", "invalid_minutes", "Estimated minutes must be between 1 and 2147483647"))
			return
		}
		revision, ok := expectedRevision(w, r, actionItemsDeleteFailure)
		if !ok {
			return
		}
		expected = &revision
	}
	if err := h.actionItems.Delete.ExecuteWithOptions(r.Context(), userID, taskID, itemID, taskusecase.DeleteActionItemOptions{
		ManualEstimatedMinutes: request.ManualEstimatedMinutes.Value,
		ManualEstimatePresent:  request.ManualEstimatedMinutes.Present,
		ExpectedTaskRevision:   expected,
	}); err != nil {
		writeActionItemError(w, actionItemsDeleteFailure, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ActionItemHandler) changeCompleted(w http.ResponseWriter, r *http.Request, spec ErrSpec, execute func(context.Context, domain.UserID, domain.TaskID, domain.ActionItemID) error, message string) {
	userID, ok := actionItemsUserID(r)
	if !ok {
		actionItemUnauthorized(w, spec)
		return
	}
	taskID, itemID := actionItemPathIDs(r)
	if !actionItemIDsRequired(w, spec, taskID, itemID, true) {
		return
	}
	if r.Body != nil {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1))
		if err != nil || len(body) > 0 {
			WriteError(w, http.StatusBadRequest, spec, ErrDetailInvalidRequestBody)
			return
		}
	}
	if err := execute(r.Context(), userID, taskID, itemID); err != nil {
		writeActionItemError(w, spec, err)
		return
	}
	WriteMessage(w, http.StatusOK, message)
}

// Complete marks an ActionItem complete.
//
//	@Summary		Complete action item
//	@Description	Marks an ActionItem complete.
//	@Tags			ActionItems
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			taskId	path		string						true	"Task ID"
//	@Param			id		path		string						true	"ActionItem ID"
//	@Param			request	body		ActionItemCompleteRequest	false	"Optional actual work time"
//	@Success		200		{object}	MessageResponse
//	@Failure		400		{object}	ErrResponse
//	@Failure		401		{object}	ErrResponse
//	@Failure		404		{object}	ErrResponse
//	@Failure		500		{object}	ErrResponse
//
//	@Router			/tasks/{taskId}/action-items/{id}:complete [post]
func (h *ActionItemHandler) Complete(w http.ResponseWriter, r *http.Request) {
	userID, ok := actionItemsUserID(r)
	if !ok {
		actionItemUnauthorized(w, actionItemsCompleteFailure)
		return
	}
	taskID, itemID := actionItemPathIDs(r)
	if !actionItemIDsRequired(w, actionItemsCompleteFailure, taskID, itemID, true) {
		return
	}
	var request ActionItemCompleteRequest
	if r.Body != nil {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			WriteError(w, http.StatusBadRequest, actionItemsCompleteFailure, ErrDetailInvalidRequestBody)
			return
		}
		if strings.TrimSpace(string(body)) != "" {
			r.Body = io.NopCloser(bytes.NewReader(body))
			if err := decodeRequest(r, &request); err != nil {
				WriteError(w, http.StatusBadRequest, actionItemsCompleteFailure, ErrDetailInvalidRequestBody)
				return
			}
		}
	}
	if err := h.actionItems.Complete.Execute(r.Context(), userID, taskID, itemID, request.ActualMinutes); err != nil {
		writeActionItemError(w, actionItemsCompleteFailure, err)
		return
	}
	WriteMessage(w, http.StatusOK, "ActionItem completed")
}

// Reopen clears ActionItem completion.
//
//	@Summary		Reopen action item
//	@Description	Reopens an ActionItem.
//	@Tags			ActionItems
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			taskId	path		string	true	"Task ID"
//	@Param			id		path		string	true	"ActionItem ID"
//	@Success		200		{object}	MessageResponse
//	@Failure		400		{object}	ErrResponse
//	@Failure		401		{object}	ErrResponse
//	@Failure		404		{object}	ErrResponse
//	@Failure		500		{object}	ErrResponse
//
//	@Router			/tasks/{taskId}/action-items/{id}:reopen [post]
func (h *ActionItemHandler) Reopen(w http.ResponseWriter, r *http.Request) {
	h.changeCompleted(w, r, actionItemsReopenFailure, h.actionItems.Reopen.Execute, "ActionItem reopened")
}

// Reorder moves an ActionItem to a zero-based position in its Task.
//
//	@Summary		Reorder action item
//	@Description	Moves the ActionItem to a new zero-based position in its Task.
//	@Tags			ActionItems
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			taskId	path		string						true	"Task ID"
//	@Param			id		path		string						true	"ActionItem ID"
//	@Param			request	body		ActionItemReorderRequest	true	"Target position"
//	@Success		200		{object}	ActionItemResponse
//	@Failure		400		{object}	ErrResponse	"Invalid position"
//	@Failure		401		{object}	ErrResponse	"Unauthorized"
//	@Failure		404		{object}	ErrResponse	"Task or action item not found"
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
	item, err := h.actionItems.Reorder.Execute(r.Context(), userID, taskID, itemID, *request.Position)
	if err != nil {
		writeActionItemError(w, actionItemsReorderFailure, err)
		return
	}
	WriteJSON(w, http.StatusOK, actionItemResponse(item))
}
