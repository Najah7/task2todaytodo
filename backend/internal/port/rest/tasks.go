package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	taskusecase "github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/Najah7/task2todaytodo/internal/port/rest/pagination"
	"github.com/go-chi/chi/v5"
)

var (
	errSpecTasksListFailed     = NewFailureErrSpec(ResourceTasks, ActionList, "Failed to list tasks")
	errSpecTasksCreateFailed   = ErrSpecTasksCreateFailed
	errSpecTasksGetFailed      = ErrSpecTasksGetFailed
	errSpecTasksUpdateFailed   = ErrSpecTasksUpdateFailed
	errSpecTasksDeleteFailed   = ErrSpecTasksDeleteFailed
	errSpecTasksStartFailed    = NewFailureErrSpec(ResourceTasks, "start", "Failed to start task")
	errSpecTasksHoldFailed     = NewFailureErrSpec(ResourceTasks, "hold", "Failed to hold task")
	errSpecTasksWaitFailed     = NewFailureErrSpec(ResourceTasks, "wait", "Failed to wait on task")
	errSpecTasksCompleteFailed = NewFailureErrSpec(ResourceTasks, "complete", "Failed to complete task")
	errSpecTasksReopenFailed   = NewFailureErrSpec(ResourceTasks, "reopen", "Failed to reopen task")
)

type TaskHandler struct {
	tasks      taskusecase.TaskUseCases
	issueID    func() string
	pageTokens *pagination.Codec
}

func NewTaskHandler(tasks taskusecase.TaskUseCases, issueID func() string, codecs ...*pagination.Codec) *TaskHandler {
	h := &TaskHandler{tasks: tasks, issueID: issueID}
	if len(codecs) > 0 {
		h.pageTokens = codecs[0]
	}
	return h
}

type TaskResponse struct {
	ID                       string  `json:"id" validate:"required"`
	UserID                   string  `json:"user_id" validate:"required"`
	AssigneeID               string  `json:"assignee_id" validate:"required"`
	Revision                 int32   `json:"revision" validate:"required"`
	ProjectID                *string `json:"project_id" validate:"required" extensions:"x-nullable"`
	ProjectName              *string `json:"project_name" validate:"required" extensions:"x-nullable"`
	Title                    string  `json:"title" validate:"required"`
	Description              string  `json:"description" validate:"required"`
	DueDate                  *string `json:"due_date" validate:"required" extensions:"x-nullable"`
	RemainingDays            *int    `json:"remaining_days" validate:"required" extensions:"x-nullable"`
	ManualEstimatedMinutes   *int    `json:"manual_estimated_minutes" validate:"required" extensions:"x-nullable"`
	EstimatedMinutes         *int    `json:"estimated_minutes" validate:"required" extensions:"x-nullable"`
	EstimateSource           string  `json:"estimate_source" validate:"required"`
	ActualMinutes            *int    `json:"actual_minutes" validate:"required" extensions:"x-nullable"`
	Progress                 int     `json:"progress" validate:"required"`
	Priority                 string  `json:"priority" validate:"required"`
	Status                   string  `json:"status" validate:"required"`
	ActionItemCount          int     `json:"action_item_count" validate:"required"`
	ActionItemCompletedCount int     `json:"action_item_completed_count" validate:"required"`
	CanUpdate                bool    `json:"can_update" validate:"required"`
	Started                  bool    `json:"started" validate:"required"`
	EstimateEditable         bool    `json:"estimate_editable" validate:"required"`
	CanAddActionItem         bool    `json:"can_add_action_item" validate:"required"`
	CanAddToToday            bool    `json:"can_add_to_today" validate:"required"`
	CreatedAt                string  `json:"created_at" validate:"required"`
	UpdatedAt                string  `json:"updated_at" validate:"required"`
}

type TaskDetailsResponse struct {
	Task TaskResponse `json:"task"`
}

type TaskListResponse struct {
	Items                    []TaskResponse `json:"items" validate:"required"`
	NextPageToken            string         `json:"next_page_token" validate:"required"`
	PreviousPageToken        string         `json:"previous_page_token" validate:"required"`
	TotalCount               int            `json:"total_count" validate:"required"`
	StatusCounts             map[string]int `json:"status_counts" validate:"required"`
	ActionItemTotalCount     int            `json:"action_item_total_count" validate:"required"`
	ActionItemCompletedCount int            `json:"action_item_completed_count" validate:"required"`
	EstimatedMinutesTotal    *int           `json:"estimated_minutes_total" validate:"required" extensions:"x-nullable"`
}

type TaskCreateRequest struct {
	Title                  string `json:"title"`
	Description            string `json:"description"`
	DueDate                string `json:"due_date"`
	ManualEstimatedMinutes *int   `json:"manual_estimated_minutes" validate:"required" minimum:"1" maximum:"2147483647"`
	Priority               string `json:"priority"`
}

type TaskUpdateRequest struct {
	Title                  optionalJSON[string] `json:"title" swaggertype:"string"`
	Description            optionalJSON[string] `json:"description" swaggertype:"string" extensions:"x-nullable"`
	DueDate                optionalJSON[string] `json:"due_date" swaggertype:"string" extensions:"x-nullable"`
	ManualEstimatedMinutes optionalJSON[int]    `json:"manual_estimated_minutes" swaggertype:"integer" extensions:"x-nullable"`
	ActualMinutes          optionalJSON[int]    `json:"actual_minutes" swaggertype:"integer" extensions:"x-nullable"`
	Priority               optionalJSON[string] `json:"priority" swaggertype:"string"`
	ProjectID              optionalJSON[string] `json:"project_id" swaggertype:"string" extensions:"x-nullable"`
}

type TaskCompleteRequest struct {
	ActualMinutes *int `json:"actual_minutes" extensions:"x-nullable" minimum:"0" maximum:"2147483647"`
}

type optionalJSON[T any] struct {
	Present bool
	Value   *T
}

func (field *optionalJSON[T]) UnmarshalJSON(data []byte) error {
	field.Present = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		field.Value = nil
		return nil
	}
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	field.Value = &value
	return nil
}

func (field optionalJSON[T]) patchField() taskusecase.PatchField[T] {
	return taskusecase.PatchField[T]{Present: field.Present, Value: field.Value}
}

func parseDate(value string) (time.Time, error) {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, err
	}
	return parsed, nil
}

func taskResponse(task dao.Task) TaskResponse {
	response := TaskResponse{
		ID:                       task.ID,
		UserID:                   task.UserID,
		AssigneeID:               task.AssigneeID,
		Revision:                 task.Revision,
		ProjectName:              projectNamePtr(task.ProjectName),
		Title:                    task.Title,
		Description:              task.Description,
		RemainingDays:            cloneInt(task.RemainingDays),
		ManualEstimatedMinutes:   cloneInt(task.ManualEstimatedMinutes),
		EstimatedMinutes:         cloneInt(task.EstimatedMinutes),
		EstimateSource:           task.EstimateSource,
		ActualMinutes:            cloneInt(task.ActualMinutes),
		Progress:                 task.Progress,
		Priority:                 task.Priority.Value,
		CanUpdate:                task.CanUpdate,
		Started:                  task.Started,
		EstimateEditable:         task.EstimateEditable,
		CanAddActionItem:         task.CanAddActionItem,
		CanAddToToday:            task.CanAddToToday,
		Status:                   task.Status.Value,
		ActionItemCount:          task.ActionItemCount,
		ActionItemCompletedCount: task.ActionItemCompletedCount,
		CreatedAt:                time.Unix(task.CreatedAt, 0).UTC().Format(time.RFC3339),
		UpdatedAt:                time.Unix(task.UpdatedAt, 0).UTC().Format(time.RFC3339),
	}
	if task.ProjectID != "" {
		projectID := task.ProjectID
		response.ProjectID = &projectID
	}
	if task.DueDate != 0 {
		dueDate := time.Unix(task.DueDate, 0).UTC().Format("2006-01-02")
		response.DueDate = &dueDate
	}
	return response
}

func projectNamePtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func (h *TaskHandler) userID(r *http.Request) (domain.UserID, bool) {
	userID, ok := r.Context().Value(UserIDContextKey).(string)
	if !ok || userID == "" {
		return "", false
	}
	return domain.UserID(userID), true
}

func taskIDFromRequest(r *http.Request) string {
	return chi.URLParam(r, "id")
}

func writeUnauthorized(w http.ResponseWriter, spec ErrSpec) {
	WriteError(w, http.StatusUnauthorized, spec, ErrDetailUnauthorized)
}

func decodeRequest(r *http.Request, target any) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(body, &object); err != nil || object == nil {
		return errors.New("request body must be a JSON object")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain one JSON value")
	}
	return nil
}

func taskErrorResponse(err error) (int, ErrDetail) {
	switch {
	case errors.Is(err, taskusecase.ErrRevisionConflict):
		return http.StatusConflict, NewErrDetail("If-Match", "revision_conflict", "The resource changed since the supplied revision")
	case errors.Is(err, taskusecase.ErrPermissionDenied):
		return http.StatusForbidden, NewErrDetail("", "permission_denied", "The required task permission is not granted")
	case errors.Is(err, taskusecase.ErrTaskNotFound):
		return http.StatusNotFound, NewErrDetail("id", "task_not_found", "Task was not found")
	case errors.Is(err, taskusecase.ErrInvalidTaskPage):
		return http.StatusBadRequest, NewErrDetail("page_size", "invalid_pagination", "Page size or cursor is invalid")
	case errors.Is(err, taskusecase.ErrTaskPatchRequiredFieldNull):
		return http.StatusBadRequest, NewErrDetail("title", "title_required", "Title cannot be null")
	case errors.Is(err, domain.ErrTaskTitleEmpty):
		return http.StatusBadRequest, NewErrDetail("title", "title_required", "Title is required")
	case errors.Is(err, domain.ErrTaskAssigneeEmpty):
		return http.StatusBadRequest, NewErrDetail("assignee_id", "required", "Assignee ID is required")
	case errors.Is(err, domain.ErrTaskPriorityEmpty), errors.Is(err, domain.ErrTaskPriorityInvalid):
		return http.StatusBadRequest, NewErrDetail("priority", "invalid_priority", "Priority is invalid")
	case errors.Is(err, domain.ErrTaskEstimatedMinutesInvalid):
		return http.StatusBadRequest, NewErrDetail("manual_estimated_minutes", "invalid_minutes", "Estimated minutes must be between 1 and 2147483647")
	case errors.Is(err, taskusecase.ErrTaskManualEstimateRequired):
		return http.StatusBadRequest, NewErrDetail("manual_estimated_minutes", "required", "A positive estimate is required for a childless Task")
	case errors.Is(err, taskusecase.ErrTaskStarted):
		return http.StatusConflict, NewErrDetail("manual_estimated_minutes", "task_started", "Task estimate cannot change after work starts or completes")
	case errors.Is(err, taskusecase.ErrTaskActualMinutesRequiresCompletion):
		return http.StatusConflict, NewErrDetail("actual_minutes", "task_not_completed", "Actual minutes can only be edited after Task completion")
	case errors.Is(err, taskusecase.ErrCannotAddActionItemToStartedTask):
		return http.StatusConflict, NewErrDetail("task_id", "task_started", "ActionItems cannot be added to a started or completed childless Task")
	case errors.Is(err, domain.ErrTaskActualMinutesInvalid):
		return http.StatusBadRequest, NewErrDetail("actual_minutes", "invalid_minutes", "Actual minutes must be zero or greater")
	case IsUniqueConstraint(err, "tasks_pkey"):
		return http.StatusConflict, NewErrDetail("id", "task_id_conflict", "Task ID already exists")
	default:
		return http.StatusInternalServerError, ErrDetailInternalServerError
	}
}

func writeTaskError(w http.ResponseWriter, spec ErrSpec, err error) {
	status, detail := taskErrorResponse(err)
	WriteError(w, status, spec, detail)
}

// Create creates a standalone task.
//
//	@Summary		Create task
//	@Description	Creates a task without project membership.
//	@Tags			Tasks
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		TaskCreateRequest	true	"Task create request"
//	@Success		201		{object}	TaskResponse
//	@Failure		400		{object}	ErrResponse	"Invalid request"
//	@Failure		401		{object}	ErrResponse	"Unauthorized"
//	@Failure		409		{object}	ErrResponse	"Task ID conflict"
//	@Failure		500		{object}	ErrResponse	"Failed to create task"
//	@Router			/tasks [post]
func (h *TaskHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.userID(r)
	if !ok {
		writeUnauthorized(w, errSpecTasksCreateFailed)
		return
	}
	var request TaskCreateRequest
	if err := decodeRequest(r, &request); err != nil {
		WriteError(w, http.StatusBadRequest, errSpecTasksCreateFailed, ErrDetailInvalidRequestBody)
		return
	}
	var dueDate time.Time
	if request.DueDate != "" {
		var err error
		dueDate, err = parseDate(request.DueDate)
		if err != nil {
			WriteError(w, http.StatusBadRequest, errSpecTasksCreateFailed, NewErrDetail("due_date", "invalid_date", "Due date must use YYYY-MM-DD format"))
			return
		}
	}
	task, err := h.tasks.Create.Execute(r.Context(), taskusecase.CreateTaskInput{
		ID:               domain.TaskID(h.issueID()),
		UserID:           userID,
		Title:            request.Title,
		Description:      request.Description,
		DueDate:          dueDate,
		EstimatedMinutes: cloneInt(request.ManualEstimatedMinutes),
		Priority:         request.Priority,
	})
	if err != nil {
		writeTaskError(w, errSpecTasksCreateFailed, err)
		return
	}
	setResourceETag(w, task.Revision)
	WriteJSON(w, http.StatusCreated, taskResponse(task))
}

// Get returns one task.
//
//	@Summary		Get task
//	@Description	Returns a task the caller may read.
//	@Tags			Tasks
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Task ID"
//	@Success		200	{object}	TaskDetailsResponse
//	@Failure		401	{object}	ErrResponse	"Unauthorized"
//	@Failure		404	{object}	ErrResponse	"Task not found"
//	@Failure		500	{object}	ErrResponse	"Failed to get task"
//	@Router			/tasks/{id} [get]
func (h *TaskHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.userID(r)
	if !ok {
		writeUnauthorized(w, errSpecTasksGetFailed)
		return
	}
	taskID := taskIDFromRequest(r)
	if taskID == "" {
		writeTaskError(w, errSpecTasksGetFailed, taskusecase.ErrTaskNotFound)
		return
	}
	task, err := h.tasks.Get.Execute(r.Context(), userID, domain.TaskID(taskID))
	if err != nil {
		writeTaskError(w, errSpecTasksGetFailed, err)
		return
	}
	setResourceETag(w, task.Revision)
	WriteJSON(w, http.StatusOK, TaskDetailsResponse{Task: taskResponse(task)})
}

// Update atomically updates Task fields.
//
//	@Param			If-Match	header		string		true	"Current task ETag, for example \"3\""
//	@Header			200			{string}	ETag		"Current task revision"
//	@Failure		428			{object}	ErrResponse	"If-Match is required"
//
//	@Summary		Update task
//	@Description	Atomically updates Task fields. ActionItems are managed through their own endpoints. If-Match uses the Task aggregate revision.
//	@Tags			Tasks
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string				true	"Task ID"
//	@Param			request	body		TaskUpdateRequest	true	"Task update request"
//	@Success		200		{object}	TaskResponse
//	@Failure		400		{object}	ErrResponse	"Invalid request"
//	@Failure		401		{object}	ErrResponse	"Unauthorized"
//	@Failure		404		{object}	ErrResponse	"Task not found"
//	@Failure		409		{object}	ErrResponse	"Task conflict"
//	@Failure		500		{object}	ErrResponse	"Failed to update task"
//	@Router			/tasks/{id} [patch]
func (h *TaskHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.userID(r)
	if !ok {
		writeUnauthorized(w, errSpecTasksUpdateFailed)
		return
	}
	expected, ok := expectedRevision(w, r, errSpecTasksUpdateFailed)
	if !ok {
		return
	}
	var request TaskUpdateRequest
	if err := decodeRequest(r, &request); err != nil {
		WriteError(w, http.StatusBadRequest, errSpecTasksUpdateFailed, ErrDetailInvalidRequestBody)
		return
	}
	var dueDate taskusecase.PatchField[time.Time]
	if request.DueDate.Present {
		dueDate.Present = true
		if request.DueDate.Value != nil {
			parsed, err := parseDate(*request.DueDate.Value)
			if err != nil {
				WriteError(w, http.StatusBadRequest, errSpecTasksUpdateFailed, NewErrDetail("due_date", "invalid_date", "Due date must use YYYY-MM-DD format"))
				return
			}
			dueDate.Value = &parsed
		}
	}
	task, err := h.tasks.Update.Execute(r.Context(), taskusecase.UpdateTaskInput{
		UserID: userID, TaskID: domain.TaskID(taskIDFromRequest(r)), ExpectedRevision: expected,
		Title: request.Title.patchField(), Description: request.Description.patchField(), DueDate: dueDate,
		ManualEstimatedMinutes: request.ManualEstimatedMinutes.patchField(), ActualMinutes: request.ActualMinutes.patchField(),
		Priority: request.Priority.patchField(), ProjectID: request.ProjectID.patchField(),
	})
	if err != nil {
		var field taskusecase.TaskUpdateFieldError
		if errors.As(err, &field) {
			code, detail := "invalid_value", "Value is invalid"
			switch field.Path {
			case "title":
				code, detail = "required", "Title is required"
			case "priority":
				code, detail = "invalid_priority", "Priority is invalid"
			case "manual_estimated_minutes", "actual_minutes":
				code, detail = "invalid_minutes", "Minutes must be zero or greater"
			case "project_id":
				code, detail = "project_not_found", "Project was not found"
			}
			WriteError(w, http.StatusBadRequest, errSpecTasksUpdateFailed, NewErrDetail(field.Path, code, detail))
			return
		}
		writeTaskError(w, errSpecTasksUpdateFailed, err)
		return
	}
	setResourceETag(w, task.Revision)
	WriteJSON(w, http.StatusOK, taskResponse(task))
}

// Delete logically deletes a task and its dependent records.
//
//	@Param			If-Match	header		string		true	"Current task ETag, for example \"3\""
//	@Failure		409			{object}	ErrResponse	"Revision conflict"
//	@Failure		428			{object}	ErrResponse	"If-Match is required"
//
//	@Summary		Delete task
//	@Description	Logically deletes a task the caller may delete and its child items and schedules.
//	@Tags			Tasks
//	@Security		BearerAuth
//	@Param			id	path	string	true	"Task ID"
//	@Success		204	"No Content"
//	@Failure		401	{object}	ErrResponse	"Unauthorized"
//	@Failure		404	{object}	ErrResponse	"Task not found"
//	@Failure		500	{object}	ErrResponse	"Failed to delete task"
//	@Router			/tasks/{id} [delete]
func (h *TaskHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.userID(r)
	if !ok {
		writeUnauthorized(w, errSpecTasksDeleteFailed)
		return
	}
	expected, ok := expectedRevision(w, r, errSpecTasksDeleteFailed)
	if !ok {
		return
	}
	if err := h.tasks.Delete.Execute(r.Context(), userID, domain.TaskID(taskIDFromRequest(r)), expected); err != nil {
		writeTaskError(w, errSpecTasksDeleteFailed, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *TaskHandler) changeStatus(w http.ResponseWriter, r *http.Request, spec ErrSpec, execute func(context.Context, domain.UserID, domain.TaskID, int32) (dao.Task, error), successMessage string) {
	userID, ok := h.userID(r)
	if !ok {
		writeUnauthorized(w, spec)
		return
	}
	taskID := taskIDFromRequest(r)
	if taskID == "" {
		writeTaskError(w, spec, taskusecase.ErrTaskNotFound)
		return
	}
	expected, ok := expectedRevision(w, r, spec)
	if !ok {
		return
	}
	task, err := execute(r.Context(), userID, domain.TaskID(taskID), expected)
	if err != nil {
		writeTaskError(w, spec, err)
		return
	}
	setResourceETag(w, task.Revision)
	WriteMessage(w, http.StatusOK, successMessage)
}

// Start moves task to in progress.
//
//	@Param			If-Match	header		string		true	"Current task ETag, for example \"3\""
//	@Header			200			{string}	ETag		"Current task revision"
//	@Failure		409			{object}	ErrResponse	"Revision conflict"
//	@Failure		428			{object}	ErrResponse	"If-Match is required"
//
//	@Summary		Start task
//	@Description	Moves task to in progress, regardless of current status.
//	@Tags			Tasks
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Task ID"
//	@Success		200	{object}	MessageResponse
//	@Failure		401	{object}	ErrResponse	"Unauthorized"
//	@Failure		404	{object}	ErrResponse	"Task not found"
//	@Failure		500	{object}	ErrResponse	"Failed to start task"
//	@Router			/tasks/{id}:start [post]
func (h *TaskHandler) Start(w http.ResponseWriter, r *http.Request) {
	h.changeStatus(w, r, errSpecTasksStartFailed, h.tasks.Start.Execute, "Task started")
}

// Hold moves task to pending.
//
//	@Param			If-Match	header		string		true	"Current task ETag, for example \"3\""
//	@Header			200			{string}	ETag		"Current task revision"
//	@Failure		409			{object}	ErrResponse	"Revision conflict"
//	@Failure		428			{object}	ErrResponse	"If-Match is required"
//
//	@Summary		Hold task
//	@Description	Moves task to pending, regardless of current status.
//	@Tags			Tasks
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Task ID"
//	@Success		200	{object}	MessageResponse
//	@Failure		401	{object}	ErrResponse	"Unauthorized"
//	@Failure		404	{object}	ErrResponse	"Task not found"
//	@Failure		500	{object}	ErrResponse	"Failed to hold task"
//	@Router			/tasks/{id}:hold [post]
func (h *TaskHandler) Hold(w http.ResponseWriter, r *http.Request) {
	h.changeStatus(w, r, errSpecTasksHoldFailed, h.tasks.Hold.Execute, "Task put on hold")
}

// Wait moves task to waiting on others.
//
//	@Param			If-Match	header		string		true	"Current task ETag, for example \"3\""
//	@Header			200			{string}	ETag		"Current task revision"
//	@Failure		409			{object}	ErrResponse	"Revision conflict"
//	@Failure		428			{object}	ErrResponse	"If-Match is required"
//
//	@Summary		Wait on task
//	@Description	Moves task to waiting on others, regardless of current status.
//	@Tags			Tasks
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Task ID"
//	@Success		200	{object}	MessageResponse
//	@Failure		401	{object}	ErrResponse	"Unauthorized"
//	@Failure		404	{object}	ErrResponse	"Task not found"
//	@Failure		500	{object}	ErrResponse	"Failed to wait on task"
//	@Router			/tasks/{id}:wait [post]
func (h *TaskHandler) Wait(w http.ResponseWriter, r *http.Request) {
	h.changeStatus(w, r, errSpecTasksWaitFailed, h.tasks.Wait.Execute, "Task marked as waiting")
}

// Complete marks task done.
//
//	@Param			If-Match	header		string		true	"Current task ETag, for example \"3\""
//	@Header			200			{string}	ETag		"Current task revision"
//	@Failure		409			{object}	ErrResponse	"Revision conflict"
//	@Failure		428			{object}	ErrResponse	"If-Match is required"
//
//	@Summary		Complete task
//	@Description	Marks task done while preserving its ActionItems.
//	@Tags			Tasks
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string				true	"Task ID"
//	@Param			request	body		TaskCompleteRequest	false	"Optional actual minutes"
//	@Success		200		{object}	MessageResponse
//	@Failure		401		{object}	ErrResponse	"Unauthorized"
//	@Failure		404		{object}	ErrResponse	"Task not found"
//	@Failure		500		{object}	ErrResponse	"Failed to complete task"
//	@Router			/tasks/{id}:complete [post]
func (h *TaskHandler) Complete(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.userID(r)
	if !ok {
		writeUnauthorized(w, errSpecTasksCompleteFailed)
		return
	}
	taskID := taskIDFromRequest(r)
	if taskID == "" {
		writeTaskError(w, errSpecTasksCompleteFailed, taskusecase.ErrTaskNotFound)
		return
	}
	expected, ok := expectedRevision(w, r, errSpecTasksCompleteFailed)
	if !ok {
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		WriteError(w, http.StatusBadRequest, errSpecTasksCompleteFailed, ErrDetailInvalidRequestBody)
		return
	}
	var request TaskCompleteRequest
	if len(bytes.TrimSpace(body)) > 0 {
		r.Body = io.NopCloser(bytes.NewReader(body))
		if err := decodeRequest(r, &request); err != nil {
			WriteError(w, http.StatusBadRequest, errSpecTasksCompleteFailed, ErrDetailInvalidRequestBody)
			return
		}
	}
	task, err := h.tasks.Complete.ExecuteWithActual(r.Context(), userID, domain.TaskID(taskID), expected, request.ActualMinutes)
	if err != nil {
		writeTaskError(w, errSpecTasksCompleteFailed, err)
		return
	}
	setResourceETag(w, task.Revision)
	WriteMessage(w, http.StatusOK, "Task completed")
}

// Reopen moves task to open.
//
//	@Param			If-Match	header		string		true	"Current task ETag, for example \"3\""
//	@Header			200			{string}	ETag		"Current task revision"
//	@Failure		409			{object}	ErrResponse	"Revision conflict"
//	@Failure		428			{object}	ErrResponse	"If-Match is required"
//
//	@Summary		Reopen task
//	@Description	Moves task to open, regardless of current status.
//	@Tags			Tasks
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Task ID"
//	@Success		200	{object}	MessageResponse
//	@Failure		401	{object}	ErrResponse	"Unauthorized"
//	@Failure		404	{object}	ErrResponse	"Task not found"
//	@Failure		500	{object}	ErrResponse	"Failed to reopen task"
//	@Router			/tasks/{id}:reopen [post]
func (h *TaskHandler) Reopen(w http.ResponseWriter, r *http.Request) {
	h.changeStatus(w, r, errSpecTasksReopenFailed, h.tasks.Reopen.Execute, "Task reopened")
}
