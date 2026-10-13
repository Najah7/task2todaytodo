package rest

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"time"

	tododomain "github.com/Najah7/task2todaytodo/internal/application/todo/domain"
	todousecase "github.com/Najah7/task2todaytodo/internal/application/todo/usecase"
	"github.com/go-chi/chi/v5"
)

var todoFailure = NewFailureErrSpec("todos", "get_today", "Failed to get today's Todo")

type TodosUseCases struct {
	GetToday         *todousecase.GetTodayUseCase
	RegenerateToday  *todousecase.RegenerateTodayUseCase
	AddActionItem    *todousecase.AddActionItemUseCase
	AddTask          *todousecase.AddTaskUseCase
	Reflow           *todousecase.ReflowUseCase
	CompleteFrame    *todousecase.CompleteFrameUseCase
	ReopenFrame      *todousecase.ReopenFrameUseCase
	RemoveActionItem *todousecase.RemoveActionItemUseCase
	HideSchedule     *todousecase.HideScheduleUseCase
	RestoreSchedule  *todousecase.RestoreScheduleUseCase
}

// RegenerateToday discards today's saved Todo rows and rebuilds the plan from
// current sources while retaining source completion state.
//
//	@Summary	Regenerate today's Todo
//	@ID			RegenerateTodayTodo
//	@Tags		Todo
//	@Produce	json
//	@Security	BearerAuth
//	@Param		If-Match	header		string	true	"Current Todo revision"
//	@Success	200			{object}	TodoTodayResponse
//	@Failure	400			{object}	ErrResponse
//	@Failure	401			{object}	ErrResponse
//	@Failure	404			{object}	ErrResponse
//	@Failure	412			{object}	ErrResponse
//	@Failure	428			{object}	ErrResponse
//	@Failure	500			{object}	ErrResponse
//	@Router		/today-todo:regenerate [post]
func (h *TodosHandler) RegenerateToday(w http.ResponseWriter, r *http.Request) {
	userID, ok := todoUserID(w, r, todoFailure)
	if !ok {
		return
	}
	expected, ok := expectedRevision(w, r, todoFailure)
	if !ok {
		return
	}
	snapshot, err := h.usecases.RegenerateToday.Execute(r.Context(), userID, expected)
	if err != nil {
		writeTodoError(w, todoFailure, err)
		return
	}
	setResourceETag(w, snapshot.Todo.Revision)
	WriteJSON(w, http.StatusOK, todoTodayResponse(snapshot))
}

type TodosHandler struct{ usecases TodosUseCases }

func NewTodosHandler(usecases TodosUseCases) *TodosHandler {
	return &TodosHandler{usecases: usecases}
}

type TodoItemResponse struct {
	ID               string  `json:"id"`
	Kind             string  `json:"kind"`
	ActionItemID     string  `json:"action_item_id,omitempty"`
	ScheduleRuleID   string  `json:"schedule_rule_id,omitempty"`
	OccurrenceDate   *string `json:"occurrence_date,omitempty"`
	TaskID           string  `json:"task_id,omitempty"`
	TaskTitle        string  `json:"task_title,omitempty"`
	ProjectID        string  `json:"project_id,omitempty"`
	ProjectTitle     string  `json:"project_title,omitempty"`
	Title            string  `json:"title"`
	Description      string  `json:"description"`
	DurationMinutes  int     `json:"duration_minutes"`
	PlannedMinutes   *int    `json:"planned_minutes" extensions:"x-nullable" minimum:"0"`
	ScheduledStart   *string `json:"scheduled_start" extensions:"x-nullable"`
	ScheduledEnd     *string `json:"scheduled_end" extensions:"x-nullable"`
	Completed        bool    `json:"completed"`
	Hidden           bool    `json:"hidden"`
	History          bool    `json:"history"`
	Position         int32   `json:"position"`
	ActualMinutes    *int    `json:"actual_minutes" extensions:"x-nullable"`
	SourceCompleted  bool    `json:"source_completed"`
	CanAddActionItem bool    `json:"can_add_action_item"`
	EstimateEditable bool    `json:"estimate_editable"`
	CanComplete      bool    `json:"can_complete"`
	CanReopen        bool    `json:"can_reopen"`
	CanMove          bool    `json:"can_move"`
	CanRemove        bool    `json:"can_remove"`
	CanHide          bool    `json:"can_hide"`
	CanRestore       bool    `json:"can_restore"`
}

type TodoProgressResponse struct {
	CompletedItems   int `json:"completed_items"`
	TotalItems       int `json:"total_items"`
	CompletedMinutes int `json:"completed_minutes"`
	TotalMinutes     int `json:"total_minutes"`
	RemainingMinutes int `json:"remaining_minutes"`
}

type TodoPolicyResponse struct {
	UrgencySameOrPast int            `json:"urgency_same_or_past"`
	Urgency1To7       int            `json:"urgency_1_to_7"`
	Urgency8To14      int            `json:"urgency_8_to_14"`
	Urgency15To30     int            `json:"urgency_15_to_30"`
	Urgency31Plus     int            `json:"urgency_31_plus"`
	UrgencyNone       int            `json:"urgency_none"`
	GapMinutes        int            `json:"gap_minutes"`
	PriorityWeights   map[string]int `json:"priority_weights"`
}

type TodoTodayResponse struct {
	ID              string               `json:"id"`
	Date            string               `json:"date"`
	Timezone        string               `json:"timezone"`
	Revision        int32                `json:"revision"`
	Items           []TodoItemResponse   `json:"items"`
	HiddenSchedules []TodoItemResponse   `json:"hidden_schedules"`
	Progress        TodoProgressResponse `json:"progress"`
	NextSchedule    *TodoItemResponse    `json:"next_schedule"`
	Policy          TodoPolicyResponse   `json:"policy"`
}

type TodoReorderRequest struct {
	ItemIDs []string `json:"item_ids" validate:"required,min=1"`
}

type TodoCompleteRequest struct {
	ActualMinutes *int `json:"actual_minutes" extensions:"x-nullable" minimum:"0" maximum:"2147483647"`
}

// GetToday returns or creates the user's execution plan for the current local day.
//
//	@Summary	Get today's Todo
//	@ID			GetTodayTodo
//	@Tags		Todo
//	@Produce	json
//	@Security	BearerAuth
//	@Success	200	{object}	TodoTodayResponse
//	@Failure	401	{object}	ErrResponse
//	@Router		/today-todo [get]
func (h *TodosHandler) GetToday(w http.ResponseWriter, r *http.Request) {
	userID, ok := todoUserID(w, r, todoFailure)
	if !ok {
		return
	}
	snapshot, err := h.usecases.GetToday.Execute(r.Context(), userID)
	if err != nil {
		writeTodoError(w, todoFailure, err)
		return
	}
	setResourceETag(w, snapshot.Todo.Revision)
	WriteJSON(w, http.StatusOK, todoTodayResponse(snapshot))
}

// AddActionItem manually adds an ActionItem to today's plan.
//
//	@Summary	Add an ActionItem to today's Todo
//	@ID			AddTodayTodoActionItem
//	@Tags		Todo
//	@Security	BearerAuth
//	@Param		actionItemId	path		string	true	"ActionItem ID"
//	@Param		If-Match		header		string	true	"Current Todo revision"
//	@Success	200				{object}	TodoTodayResponse
//	@Router		/today-todo/action-items/{actionItemId} [post]
func (h *TodosHandler) AddActionItem(w http.ResponseWriter, r *http.Request) {
	userID, ok := todoUserID(w, r, todoFailure)
	if !ok {
		return
	}
	expected, ok := expectedRevision(w, r, todoFailure)
	if !ok {
		return
	}
	if _, err := h.usecases.AddActionItem.Execute(r.Context(), userID, chi.URLParam(r, "actionItemId"), expected); err != nil {
		writeTodoError(w, todoFailure, err)
		return
	}
	h.writeToday(w, r, userID)
}

// AddTask manually adds a childless Task to today's plan.
//
//	@Summary	Add a Task to today's Todo
//	@ID			AddTodayTodoTask
//	@Tags		Todo
//	@Security	BearerAuth
//	@Param		taskId		path		string	true	"Task ID"
//	@Param		If-Match	header		string	true	"Current Todo revision"
//	@Success	200			{object}	TodoTodayResponse
//	@Router		/today-todo/tasks/{taskId} [post]
func (h *TodosHandler) AddTask(w http.ResponseWriter, r *http.Request) {
	userID, ok := todoUserID(w, r, todoFailure)
	if !ok {
		return
	}
	expected, ok := expectedRevision(w, r, todoFailure)
	if !ok {
		return
	}
	taskID := chi.URLParam(r, "taskId")
	if taskID == "" {
		writeTodoError(w, todoFailure, todousecase.ErrNotFound)
		return
	}
	if _, err := h.usecases.AddTask.Execute(r.Context(), userID, taskID, expected); err != nil {
		writeTodoError(w, todoFailure, err)
		return
	}
	h.writeToday(w, r, userID)
}

// Reorder persists only the requested display positions.
//
//	@Summary	Reorder today's Todo items
//	@ID			ReorderTodayTodo
//	@Tags		Todo
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		If-Match	header		string				true	"Current Todo revision"
//	@Param		request		body		TodoReorderRequest	true	"Ordered item IDs"
//	@Success	200			{object}	TodoTodayResponse
//	@Router		/today-todo:reorder [post]
func (h *TodosHandler) Reorder(w http.ResponseWriter, r *http.Request) {
	userID, ok := todoUserID(w, r, todoFailure)
	if !ok {
		return
	}
	expected, ok := expectedRevision(w, r, todoFailure)
	if !ok {
		return
	}
	var request TodoReorderRequest
	if err := decodeTodoRequest(r, &request, false); err != nil || len(request.ItemIDs) == 0 {
		WriteError(w, http.StatusBadRequest, todoFailure, ErrDetailInvalidRequestBody)
		return
	}
	if _, err := h.usecases.Reflow.Execute(r.Context(), userID, expected, request.ItemIDs); err != nil {
		writeTodoError(w, todoFailure, err)
		return
	}
	h.writeToday(w, r, userID)
}

// CompleteFrame completes one ActionItem work frame or Schedule occurrence.
//
//	@Summary	Complete a Todo frame
//	@ID			CompleteTodayTodoItem
//	@Tags		Todo
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		itemId		path		string				true	"Todo item ID"
//	@Param		If-Match	header		string				true	"Current Todo revision"
//	@Param		request		body		TodoCompleteRequest	false	"Optional actual minutes for whole-source completion"
//	@Success	200			{object}	TodoTodayResponse
//	@Router		/today-todo/items/{itemId}:complete [post]
func (h *TodosHandler) CompleteFrame(w http.ResponseWriter, r *http.Request) {
	userID, ok := todoUserID(w, r, todoFailure)
	if !ok {
		return
	}
	expected, ok := expectedRevision(w, r, todoFailure)
	if !ok {
		return
	}
	var request TodoCompleteRequest
	if err := decodeTodoRequest(r, &request, true); err != nil {
		writeTodoError(w, todoFailure, tododomain.ErrInvalidInput)
		return
	}
	if _, err := h.usecases.CompleteFrame.Execute(r.Context(), userID, chi.URLParam(r, "itemId"), expected, request.ActualMinutes); err != nil {
		writeTodoError(w, todoFailure, err)
		return
	}
	h.writeToday(w, r, userID)
}

// decodeTodoRequest shares the strict object/field/trailing-value rules used by
// the other REST request bodies, while allowing an empty body for completion.
func decodeTodoRequest(r *http.Request, target any, allowEmpty bool) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if allowEmpty && len(bytes.TrimSpace(body)) == 0 {
		return nil
	}
	request := r.Clone(r.Context())
	request.Body = io.NopCloser(bytes.NewReader(body))
	return decodeRequest(request, target)
}

// ReopenFrame reopens one completed Todo frame.
//
//	@Summary	Reopen a Todo frame
//	@ID			ReopenTodayTodoItem
//	@Tags		Todo
//	@Security	BearerAuth
//	@Param		itemId		path		string	true	"Todo item ID"
//	@Param		If-Match	header		string	true	"Current Todo revision"
//	@Success	200			{object}	TodoTodayResponse
//	@Router		/today-todo/items/{itemId}:reopen [post]
func (h *TodosHandler) ReopenFrame(w http.ResponseWriter, r *http.Request) {
	userID, ok := todoUserID(w, r, todoFailure)
	if !ok {
		return
	}
	expected, ok := expectedRevision(w, r, todoFailure)
	if !ok {
		return
	}
	if _, err := h.usecases.ReopenFrame.Execute(r.Context(), userID, chi.URLParam(r, "itemId"), expected); err != nil {
		writeTodoError(w, todoFailure, err)
		return
	}
	h.writeToday(w, r, userID)
}

// RemoveActionItem removes one unstarted frame from today's plan.
//
//	@Summary	Remove an ActionItem frame
//	@ID			RemoveTodayTodoItem
//	@Tags		Todo
//	@Security	BearerAuth
//	@Param		itemId		path		string	true	"Todo item ID"
//	@Param		If-Match	header		string	true	"Current Todo revision"
//	@Success	200			{object}	TodoTodayResponse
//	@Router		/today-todo/items/{itemId}:remove [post]
func (h *TodosHandler) RemoveActionItem(w http.ResponseWriter, r *http.Request) {
	userID, ok := todoUserID(w, r, todoFailure)
	if !ok {
		return
	}
	expected, ok := expectedRevision(w, r, todoFailure)
	if !ok {
		return
	}
	if _, err := h.usecases.RemoveActionItem.Execute(r.Context(), userID, chi.URLParam(r, "itemId"), expected); err != nil {
		writeTodoError(w, todoFailure, err)
		return
	}
	h.writeToday(w, r, userID)
}

// HideSchedule hides a scheduled block from today's display without changing work rows.
//
//	@Summary	Hide a Schedule frame
//	@ID			HideTodayTodoSchedule
//	@Tags		Todo
//	@Security	BearerAuth
//	@Param		itemId		path		string	true	"Todo item ID"
//	@Param		If-Match	header		string	true	"Current Todo revision"
//	@Success	200			{object}	TodoTodayResponse
//	@Router		/today-todo/schedules/{itemId}:hide [post]
func (h *TodosHandler) HideSchedule(w http.ResponseWriter, r *http.Request) {
	userID, ok := todoUserID(w, r, todoFailure)
	if !ok {
		return
	}
	expected, ok := expectedRevision(w, r, todoFailure)
	if !ok {
		return
	}
	if _, err := h.usecases.HideSchedule.Execute(r.Context(), userID, chi.URLParam(r, "itemId"), expected); err != nil {
		writeTodoError(w, todoFailure, err)
		return
	}
	h.writeToday(w, r, userID)
}

// RestoreSchedule restores a hidden Schedule and inserts it by its internal start time.
//
//	@Summary	Restore a hidden Schedule
//	@ID			RestoreTodayTodoSchedule
//	@Tags		Todo
//	@Security	BearerAuth
//	@Param		itemId		path		string	true	"Todo item ID"
//	@Param		If-Match	header		string	true	"Current Todo revision"
//	@Success	200			{object}	TodoTodayResponse
//	@Router		/today-todo/schedules/{itemId}:restore [post]
func (h *TodosHandler) RestoreSchedule(w http.ResponseWriter, r *http.Request) {
	userID, ok := todoUserID(w, r, todoFailure)
	if !ok {
		return
	}
	expected, ok := expectedRevision(w, r, todoFailure)
	if !ok {
		return
	}
	if _, err := h.usecases.RestoreSchedule.Execute(r.Context(), userID, chi.URLParam(r, "itemId"), expected); err != nil {
		writeTodoError(w, todoFailure, err)
		return
	}
	h.writeToday(w, r, userID)
}

func (h *TodosHandler) writeToday(w http.ResponseWriter, r *http.Request, userID string) {
	snapshot, err := h.usecases.GetToday.Execute(r.Context(), userID)
	if err != nil {
		writeTodoError(w, todoFailure, err)
		return
	}
	setResourceETag(w, snapshot.Todo.Revision)
	WriteJSON(w, http.StatusOK, todoTodayResponse(snapshot))
}

func todoUserID(w http.ResponseWriter, r *http.Request, spec ErrSpec) (string, bool) {
	userID, ok := r.Context().Value(UserIDContextKey).(string)
	if !ok || userID == "" {
		WriteError(w, http.StatusUnauthorized, spec, ErrDetailUnauthorized)
		return "", false
	}
	return userID, true
}

func todoTodayResponse(snapshot todousecase.TodaySnapshot) TodoTodayResponse {
	return TodoTodayResponse{ID: snapshot.Todo.ID, Date: snapshot.Todo.Date.Format("2006-01-02"), Timezone: snapshot.Todo.Timezone,
		Revision: snapshot.Todo.Revision, Items: mapTodoItems(snapshot.Items), HiddenSchedules: mapTodoItems(snapshot.HiddenSchedules),
		Progress: TodoProgressResponse{CompletedItems: snapshot.Progress.CompletedItems, TotalItems: snapshot.Progress.TotalItems,
			CompletedMinutes: snapshot.Progress.CompletedMinutes, TotalMinutes: snapshot.Progress.TotalMinutes, RemainingMinutes: snapshot.Progress.RemainingMinutes},
		NextSchedule: mapTodoItemPointer(snapshot.NextSchedule), Policy: TodoPolicyResponse{
			UrgencySameOrPast: snapshot.Policy.UrgencySameOrPast, Urgency1To7: snapshot.Policy.Urgency1To7, Urgency8To14: snapshot.Policy.Urgency8To14,
			Urgency15To30: snapshot.Policy.Urgency15To30, Urgency31Plus: snapshot.Policy.Urgency31Plus, UrgencyNone: snapshot.Policy.UrgencyNone, GapMinutes: snapshot.Policy.GapMinutes,
			PriorityWeights: snapshot.Policy.PriorityWeights,
		}}
}

func mapTodoItems(items []todousecase.TodayItem) []TodoItemResponse {
	result := make([]TodoItemResponse, 0, len(items))
	for _, item := range items {
		result = append(result, mapTodoItem(item))
	}
	return result
}

func mapTodoItemPointer(item *todousecase.TodayItem) *TodoItemResponse {
	if item == nil {
		return nil
	}
	result := mapTodoItem(*item)
	return &result
}

func mapTodoItem(item todousecase.TodayItem) TodoItemResponse {
	result := TodoItemResponse{ID: item.ID, Kind: item.Kind, ActionItemID: item.ActionItemID, ScheduleRuleID: item.ScheduleRuleID,
		TaskID: item.TaskID, TaskTitle: item.TaskTitle, ProjectID: item.ProjectID, ProjectTitle: item.ProjectTitle,
		Title: item.Title, Description: item.Description, DurationMinutes: item.DurationMinutes, PlannedMinutes: item.PlannedMinutes, Completed: item.Completed,
		Hidden: item.Hidden, History: item.History, Position: item.Position, ActualMinutes: item.ActualMinutes,
		SourceCompleted:  item.SourceCompleted,
		CanAddActionItem: item.CanAddActionItem, EstimateEditable: item.EstimateEditable,
		CanComplete: item.CanComplete, CanReopen: item.CanReopen,
		CanMove: item.CanMove, CanRemove: item.CanRemove, CanHide: item.CanHide, CanRestore: item.CanRestore}
	if !item.OccurrenceDate.IsZero() {
		date := item.OccurrenceDate.Format("2006-01-02")
		result.OccurrenceDate = &date
	}
	if item.Kind == string(tododomain.FrameSchedule) {
		result.ScheduledStart = formatTodoTime(item.ScheduledStart)
		result.ScheduledEnd = formatTodoTime(item.ScheduledEnd)
	}
	return result
}

func formatTodoTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format(time.RFC3339Nano)
	return &formatted
}

func writeTodoError(w http.ResponseWriter, spec ErrSpec, err error) {
	switch {
	case errors.Is(err, todousecase.ErrRevisionConflict):
		WriteError(w, http.StatusPreconditionFailed, spec, NewErrDetail("If-Match", "revision_mismatch", "The Todo changed; fetch the latest version and retry"))
	case errors.Is(err, todousecase.ErrPermissionDenied):
		WriteError(w, http.StatusForbidden, spec, NewErrDetail("", "permission_denied", "The caller cannot perform this Todo operation"))
	case errors.Is(err, todousecase.ErrTaskNotFound):
		WriteError(w, http.StatusNotFound, spec, NewErrDetail("task_id", "task_not_found", "Task was not found or is not eligible for a direct Todo frame"))
	case errors.Is(err, todousecase.ErrNotFound), errors.Is(err, todousecase.ErrFrameNotFound), errors.Is(err, todousecase.ErrActionItemNotFound):
		WriteError(w, http.StatusNotFound, spec, NewErrDetail("", "not_found", "Todo item was not found"))
	case errors.Is(err, todousecase.ErrTaskStarted):
		WriteError(w, http.StatusConflict, spec, NewErrDetail("", "task_started", "Started Task frames cannot be removed"))
	case errors.Is(err, todousecase.ErrActionItemStarted):
		WriteError(w, http.StatusConflict, spec, NewErrDetail("", "action_item_started", "Started ActionItems cannot be removed"))
	case errors.Is(err, todousecase.ErrAllDayBlocksAction):
		WriteError(w, http.StatusConflict, spec, NewErrDetail("", "all_day_schedule_blocks_action_items", "ActionItems cannot be added while an all-day Schedule occupies today"))
	case errors.Is(err, todousecase.ErrFrameCompleted), errors.Is(err, todousecase.ErrScheduleCompleted):
		WriteError(w, http.StatusConflict, spec, NewErrDetail("", "invalid_state", "This completed Todo item cannot be changed"))
	case errors.Is(err, tododomain.ErrInvalidReflow), errors.Is(err, tododomain.ErrInvalidInput), errors.Is(err, tododomain.ErrInvalidTodoItem):
		WriteError(w, http.StatusBadRequest, spec, ErrDetailInvalidRequestBody)
	default:
		WriteError(w, http.StatusInternalServerError, spec, ErrDetailInternalServerError)
	}
}
