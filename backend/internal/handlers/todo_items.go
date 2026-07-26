package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Najah7/task2todaytodo/internal/domain/shared"
	domain "github.com/Najah7/task2todaytodo/internal/domain/task"
)

type TodoItemHandler struct {
	svc   *domain.TodoItemService
	idGen shared.IDGenerator
}

func NewTodoItemHandler(svc *domain.TodoItemService, idGen shared.IDGenerator) *TodoItemHandler {
	return &TodoItemHandler{svc: svc, idGen: idGen}
}

type TodoItemCreateRequest struct {
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	DueDate       *string  `json:"due_date" format:"date"`
	Position      *int     `json:"position"`
	IntervalWeeks *int     `json:"interval_weeks"`
	Frequencies   []string `json:"frequencies"`
}

// Create godoc
//
//	@Summary		Create todo item
//	@Description	Creates a TodoItem under an owned Task.
//	@Tags			TodoItems
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			task_id	path		string					true	"Task ID"
//	@Param			request	body		TodoItemCreateRequest	true	"TodoItem create request"
//	@Success		201		{object}	TodoItemResponse
//	@Failure		400		{object}	ErrResponse	"Invalid request body"
//	@Failure		401		{object}	ErrResponse	"Unauthorized"
//	@Failure		404		{object}	ErrResponse	"Task not found"
//	@Failure		409		{object}	ErrResponse	"Position conflict"
//	@Failure		500		{object}	ErrResponse	"Failed to create todo item"
//	@Router			/tasks/{task_id}/todo-items [post]
func (h *TodoItemHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := taskUserIDFromRequest(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, ErrSpecTodoItemsCreateFailed, ErrDetailUnauthorized)
		return
	}

	var req TodoItemCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, ErrSpecTodoItemsCreateFailed, ErrDetailInvalidRequestBody)
		return
	}

	dueDate, err := parseOptionalDateOnly(req.DueDate)
	if err != nil {
		WriteError(w, http.StatusBadRequest, ErrSpecTodoItemsCreateFailed, errDetailInvalidDateOnly("due_date"))
		return
	}

	item, err := h.svc.Create(r.Context(), h.idGen.Generate, domain.TodoItemCreateParams{
		UserID:        userID,
		TaskID:        taskIDFromRequest(r),
		Title:         req.Title,
		Description:   req.Description,
		DueDate:       dueDate,
		Position:      req.Position,
		IntervalWeeks: req.IntervalWeeks,
		Frequencies:   req.Frequencies,
	})
	if err != nil {
		status, detail := todoItemErrToErrResponse(err)
		WriteError(w, status, ErrSpecTodoItemsCreateFailed, detail)
		return
	}

	WriteJSON(w, http.StatusCreated, newTodoItemResponse(item))
}

// Check godoc
//
//	@Summary		Check todo item
//	@Description	Marks an owned TodoItem as completed.
//	@Tags			TodoItems
//	@Produce		json
//	@Security		BearerAuth
//	@Param			task_id			path		string			true	"Task ID"
//	@Param			todo_item_id	path		string			true	"TodoItem ID"
//	@Success		200				{object}	MessageResponse	"OK"
//	@Failure		401				{object}	ErrResponse		"Unauthorized"
//	@Failure		404				{object}	ErrResponse		"TodoItem not found"
//	@Failure		500				{object}	ErrResponse		"Failed to update todo item"
//	@Router			/tasks/{task_id}/todo-items/{todo_item_id}:checked [post]
func (h *TodoItemHandler) Check(w http.ResponseWriter, r *http.Request) {
	h.setCompleted(w, r, true)
}

// Uncheck godoc
//
//	@Summary		Uncheck todo item
//	@Description	Marks an owned TodoItem as incomplete.
//	@Tags			TodoItems
//	@Produce		json
//	@Security		BearerAuth
//	@Param			task_id			path		string			true	"Task ID"
//	@Param			todo_item_id	path		string			true	"TodoItem ID"
//	@Success		200				{object}	MessageResponse	"OK"
//	@Failure		401				{object}	ErrResponse		"Unauthorized"
//	@Failure		404				{object}	ErrResponse		"TodoItem not found"
//	@Failure		500				{object}	ErrResponse		"Failed to update todo item"
//	@Router			/tasks/{task_id}/todo-items/{todo_item_id}:unchecked [post]
func (h *TodoItemHandler) Uncheck(w http.ResponseWriter, r *http.Request) {
	h.setCompleted(w, r, false)
}

func (h *TodoItemHandler) setCompleted(w http.ResponseWriter, r *http.Request, completed bool) {
	userID, ok := taskUserIDFromRequest(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, ErrSpecTodoItemsUpdateFailed, ErrDetailUnauthorized)
		return
	}

	var err error
	if completed {
		err = h.svc.Check(r.Context(), userID, taskIDFromRequest(r), todoItemIDFromRequest(r))
	} else {
		err = h.svc.Uncheck(r.Context(), userID, taskIDFromRequest(r), todoItemIDFromRequest(r))
	}
	if err != nil {
		status, detail := todoItemErrToErrResponse(err)
		WriteError(w, status, ErrSpecTodoItemsUpdateFailed, detail)
		return
	}

	WriteMessage(w, http.StatusOK, "OK")
}

// Delete godoc
//
//	@Summary		Delete todo item
//	@Description	Deletes an owned TodoItem.
//	@Tags			TodoItems
//	@Produce		json
//	@Security		BearerAuth
//	@Param			task_id			path		string			true	"Task ID"
//	@Param			todo_item_id	path		string			true	"TodoItem ID"
//	@Success		200				{object}	MessageResponse	"OK"
//	@Failure		401				{object}	ErrResponse		"Unauthorized"
//	@Failure		404				{object}	ErrResponse		"TodoItem not found"
//	@Failure		500				{object}	ErrResponse		"Failed to delete todo item"
//	@Router			/tasks/{task_id}/todo-items/{todo_item_id} [delete]
func (h *TodoItemHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := taskUserIDFromRequest(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, ErrSpecTodoItemsDeleteFailed, ErrDetailUnauthorized)
		return
	}

	if err := h.svc.Delete(r.Context(), userID, taskIDFromRequest(r), todoItemIDFromRequest(r)); err != nil {
		status, detail := todoItemErrToErrResponse(err)
		WriteError(w, status, ErrSpecTodoItemsDeleteFailed, detail)
		return
	}

	WriteMessage(w, http.StatusOK, "OK")
}

func todoItemErrToErrResponse(err error) (int, ErrDetail) {
	switch {
	case errors.Is(err, domain.ErrTodoItemTaskNotFound):
		return http.StatusNotFound, NewErrDetail("task_id", "task_not_found", "Task not found")
	case errors.Is(err, domain.ErrTodoItemNotFound):
		return http.StatusNotFound, NewErrDetail("todo_item_id", "todo_item_not_found", "Todo item not found")
	case errors.Is(err, domain.ErrTodoItemPositionConflict):
		return http.StatusConflict, NewErrDetail("position", "todo_item_position_conflict", "Todo item position already exists")
	case errors.Is(err, domain.ErrTodoItemTitleEmpty):
		return http.StatusBadRequest, NewErrDetail("title", "todo_item_title_required", "Todo item title is required")
	case errors.Is(err, domain.ErrTodoItemPositionLess):
		return http.StatusBadRequest, NewErrDetail("position", "invalid_todo_item_position", "Todo item position must be greater than or equal to 0")
	case errors.Is(err, domain.ErrTodoItemIntervalWeeksLess):
		return http.StatusBadRequest, NewErrDetail("interval_weeks", "invalid_interval_weeks", "Interval weeks must be greater than or equal to 0")
	case errors.Is(err, domain.ErrTaskFrequencyInvalid), errors.Is(err, domain.ErrTaskFrequencyEmpty):
		return http.StatusBadRequest, NewErrDetail("frequencies", "invalid_task_frequency", "Frequencies must be supported weekday values")
	default:
		return http.StatusInternalServerError, ErrDetailInternalServerError
	}
}
