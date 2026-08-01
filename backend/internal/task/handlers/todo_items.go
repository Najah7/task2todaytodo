package handlers

import (
	"encoding/json"
	"errors"
	sharedhandlers "github.com/Najah7/task2todaytodo/internal/shared/handlers"
	"net/http"

	"github.com/Najah7/task2todaytodo/internal/shared"
	domain "github.com/Najah7/task2todaytodo/internal/task/domain"
	taskusecase "github.com/Najah7/task2todaytodo/internal/task/usecase"
)

type TodoItemHandler struct {
	svc   *taskusecase.TodoItemService
	idGen shared.IDGenerator
}

func NewTodoItemHandler(svc *taskusecase.TodoItemService, idGen shared.IDGenerator) *TodoItemHandler {
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
//	@Failure		400		{object}	sharedhandlers.ErrResponse	"Invalid request body"
//	@Failure		401		{object}	sharedhandlers.ErrResponse	"Unauthorized"
//	@Failure		404		{object}	sharedhandlers.ErrResponse	"Task not found"
//	@Failure		409		{object}	sharedhandlers.ErrResponse	"Position conflict"
//	@Failure		500		{object}	sharedhandlers.ErrResponse	"Failed to create todo item"
//	@Router			/tasks/{task_id}/todo-items [post]
func (h *TodoItemHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := taskUserIDFromRequest(r)
	if !ok {
		sharedhandlers.WriteError(w, http.StatusUnauthorized, sharedhandlers.ErrSpecTodoItemsCreateFailed, sharedhandlers.ErrDetailUnauthorized)
		return
	}

	var req TodoItemCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sharedhandlers.WriteError(w, http.StatusBadRequest, sharedhandlers.ErrSpecTodoItemsCreateFailed, sharedhandlers.ErrDetailInvalidRequestBody)
		return
	}

	dueDate, err := parseOptionalDateOnly(req.DueDate)
	if err != nil {
		sharedhandlers.WriteError(w, http.StatusBadRequest, sharedhandlers.ErrSpecTodoItemsCreateFailed, errDetailInvalidDateOnly("due_date"))
		return
	}

	item, err := h.svc.Create(r.Context(), h.idGen.Generate, taskusecase.TodoItemCreateParams{
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
		sharedhandlers.WriteError(w, status, sharedhandlers.ErrSpecTodoItemsCreateFailed, detail)
		return
	}

	sharedhandlers.WriteJSON(w, http.StatusCreated, newTodoItemResponse(item))
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
//	@Failure		401				{object}	sharedhandlers.ErrResponse		"Unauthorized"
//	@Failure		404				{object}	sharedhandlers.ErrResponse		"TodoItem not found"
//	@Failure		500				{object}	sharedhandlers.ErrResponse		"Failed to update todo item"
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
//	@Failure		401				{object}	sharedhandlers.ErrResponse		"Unauthorized"
//	@Failure		404				{object}	sharedhandlers.ErrResponse		"TodoItem not found"
//	@Failure		500				{object}	sharedhandlers.ErrResponse		"Failed to update todo item"
//	@Router			/tasks/{task_id}/todo-items/{todo_item_id}:unchecked [post]
func (h *TodoItemHandler) Uncheck(w http.ResponseWriter, r *http.Request) {
	h.setCompleted(w, r, false)
}

func (h *TodoItemHandler) setCompleted(w http.ResponseWriter, r *http.Request, completed bool) {
	userID, ok := taskUserIDFromRequest(r)
	if !ok {
		sharedhandlers.WriteError(w, http.StatusUnauthorized, sharedhandlers.ErrSpecTodoItemsUpdateFailed, sharedhandlers.ErrDetailUnauthorized)
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
		sharedhandlers.WriteError(w, status, sharedhandlers.ErrSpecTodoItemsUpdateFailed, detail)
		return
	}

	sharedhandlers.WriteMessage(w, http.StatusOK, "OK")
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
//	@Failure		401				{object}	sharedhandlers.ErrResponse		"Unauthorized"
//	@Failure		404				{object}	sharedhandlers.ErrResponse		"TodoItem not found"
//	@Failure		500				{object}	sharedhandlers.ErrResponse		"Failed to delete todo item"
//	@Router			/tasks/{task_id}/todo-items/{todo_item_id} [delete]
func (h *TodoItemHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := taskUserIDFromRequest(r)
	if !ok {
		sharedhandlers.WriteError(w, http.StatusUnauthorized, sharedhandlers.ErrSpecTodoItemsDeleteFailed, sharedhandlers.ErrDetailUnauthorized)
		return
	}

	if err := h.svc.Delete(r.Context(), userID, taskIDFromRequest(r), todoItemIDFromRequest(r)); err != nil {
		status, detail := todoItemErrToErrResponse(err)
		sharedhandlers.WriteError(w, status, sharedhandlers.ErrSpecTodoItemsDeleteFailed, detail)
		return
	}

	sharedhandlers.WriteMessage(w, http.StatusOK, "OK")
}

func todoItemErrToErrResponse(err error) (int, sharedhandlers.ErrDetail) {
	switch {
	case errors.Is(err, taskusecase.ErrTodoItemTaskNotFound):
		return http.StatusNotFound, sharedhandlers.NewErrDetail("task_id", "task_not_found", "Task not found")
	case errors.Is(err, taskusecase.ErrTodoItemNotFound):
		return http.StatusNotFound, sharedhandlers.NewErrDetail("todo_item_id", "todo_item_not_found", "Todo item not found")
	case errors.Is(err, taskusecase.ErrTodoItemPositionConflict):
		return http.StatusConflict, sharedhandlers.NewErrDetail("position", "todo_item_position_conflict", "Todo item position already exists")
	case errors.Is(err, domain.ErrTodoItemTitleEmpty):
		return http.StatusBadRequest, sharedhandlers.NewErrDetail("title", "todo_item_title_required", "Todo item title is required")
	case errors.Is(err, domain.ErrTodoItemPositionLess):
		return http.StatusBadRequest, sharedhandlers.NewErrDetail("position", "invalid_todo_item_position", "Todo item position must be greater than or equal to 0")
	case errors.Is(err, domain.ErrTodoItemIntervalWeeksLess):
		return http.StatusBadRequest, sharedhandlers.NewErrDetail("interval_weeks", "invalid_interval_weeks", "Interval weeks must be greater than or equal to 0")
	case errors.Is(err, domain.ErrTaskFrequencyInvalid), errors.Is(err, domain.ErrTaskFrequencyEmpty):
		return http.StatusBadRequest, sharedhandlers.NewErrDetail("frequencies", "invalid_task_frequency", "Frequencies must be supported weekday values")
	default:
		return http.StatusInternalServerError, sharedhandlers.ErrDetailInternalServerError
	}
}
