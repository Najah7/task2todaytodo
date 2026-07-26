package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	auth "github.com/Najah7/task2todaytodo/internal/auth/domain"
	"github.com/Najah7/task2todaytodo/internal/shared"
	domain "github.com/Najah7/task2todaytodo/internal/task/domain"
	taskusecase "github.com/Najah7/task2todaytodo/internal/task/usecase"
	"github.com/go-chi/chi/v5"
)

type TaskHandler struct {
	svc   *taskusecase.TaskService
	idGen shared.IDGenerator
}

func NewTaskHandler(svc *taskusecase.TaskService, idGen shared.IDGenerator) *TaskHandler {
	return &TaskHandler{svc: svc, idGen: idGen}
}

type TaskResponse struct {
	ID               string    `json:"id"`
	ProjectID        string    `json:"project_id"`
	Title            string    `json:"title"`
	Description      string    `json:"description"`
	DueDate          *string   `json:"due_date" format:"date"`
	EstimatedMinutes *int      `json:"estimated_minutes"`
	ActualMinutes    *int      `json:"actual_minutes"`
	Progress         int       `json:"progress"`
	Priority         string    `json:"priority"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type TaskAggregateResponse struct {
	TaskResponse
	TodoItems     []TodoItemResponse     `json:"todo_items"`
	TaskSchedules []TaskScheduleResponse `json:"task_schedules"`
}

type TodoItemResponse struct {
	ID            string    `json:"id"`
	TaskID        string    `json:"task_id"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	DueDate       *string   `json:"due_date" format:"date"`
	Completed     bool      `json:"completed"`
	Position      int       `json:"position"`
	IntervalWeeks int       `json:"interval_weeks"`
	Frequencies   []string  `json:"frequencies"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type TaskScheduleResponse struct {
	ID            string    `json:"id"`
	TaskID        string    `json:"task_id"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	Location      string    `json:"location"`
	IntervalWeeks int       `json:"interval_weeks"`
	Frequencies   []string  `json:"frequencies"`
	StartAt       time.Time `json:"start_at"`
	EndAt         time.Time `json:"end_at"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func newTaskResponse(task domain.Task) TaskResponse {
	return TaskResponse{
		ID:               string(task.ID),
		ProjectID:        string(task.ProjectID),
		Title:            task.Title,
		Description:      task.Description,
		DueDate:          dateOnlyResponse(task.DueDate),
		EstimatedMinutes: task.EstimatedMinutes,
		ActualMinutes:    task.ActualMinutes,
		Progress:         task.Progress,
		Priority:         task.Priority.String(),
		Status:           task.Status.String(),
		CreatedAt:        task.CreatedAt,
		UpdatedAt:        task.UpdatedAt,
	}
}

func newTaskAggregateResponse(aggregate domain.TaskAggregate) TaskAggregateResponse {
	todoItems := make([]TodoItemResponse, 0, len(aggregate.TodoItems))
	for _, item := range aggregate.TodoItems {
		todoItems = append(todoItems, newTodoItemResponse(item))
	}

	schedules := make([]TaskScheduleResponse, 0, len(aggregate.TaskSchedules))
	for _, schedule := range aggregate.TaskSchedules {
		schedules = append(schedules, newTaskScheduleResponse(schedule))
	}

	return TaskAggregateResponse{
		TaskResponse:  newTaskResponse(aggregate.Task),
		TodoItems:     todoItems,
		TaskSchedules: schedules,
	}
}

func newTodoItemResponse(item domain.TodoItem) TodoItemResponse {
	return TodoItemResponse{
		ID:            string(item.ID),
		TaskID:        string(item.TaskID),
		Title:         item.Title,
		Description:   item.Description,
		DueDate:       dateOnlyResponse(item.DueDate),
		Completed:     item.Completed,
		Position:      item.Position,
		IntervalWeeks: item.IntervalWeeks,
		Frequencies:   taskFrequencyValues(item.Frequencies),
		CreatedAt:     item.CreatedAt,
		UpdatedAt:     item.UpdatedAt,
	}
}

func newTaskScheduleResponse(schedule domain.TaskSchedule) TaskScheduleResponse {
	return TaskScheduleResponse{
		ID:            string(schedule.ID),
		TaskID:        string(schedule.TaskID),
		Title:         schedule.Title,
		Description:   schedule.Description,
		Location:      schedule.Location,
		IntervalWeeks: schedule.IntervalWeeks,
		Frequencies:   taskFrequencyValues(schedule.Frequencies),
		StartAt:       schedule.StartAt,
		EndAt:         schedule.EndAt,
		CreatedAt:     schedule.CreatedAt,
		UpdatedAt:     schedule.UpdatedAt,
	}
}

func taskFrequencyValues(frequencies domain.TaskFrequencies) []string {
	values := make([]string, 0, len(frequencies))
	for _, frequency := range frequencies {
		values = append(values, frequency.String())
	}
	return values
}

type TaskCreateRequest struct {
	Title            string  `json:"title"`
	Description      string  `json:"description"`
	DueDate          *string `json:"due_date" format:"date"`
	EstimatedMinutes *int    `json:"estimated_minutes"`
	ActualMinutes    *int    `json:"actual_minutes"`
	Priority         string  `json:"priority"`
	Status           string  `json:"status"`
}

// Create godoc
//
//	@Summary		Create task
//	@Description	Creates a standalone Task for the authenticated user.
//	@Tags			Tasks
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		TaskCreateRequest	true	"Task create request"
//	@Success		201		{object}	TaskResponse
//	@Failure		400		{object}	ErrResponse	"Invalid request body"
//	@Failure		401		{object}	ErrResponse	"Unauthorized"
//	@Failure		500		{object}	ErrResponse	"Failed to create task"
//	@Router			/tasks [post]
func (h *TaskHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := taskUserIDFromRequest(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, ErrSpecTasksCreateFailed, ErrDetailUnauthorized)
		return
	}

	var req TaskCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, ErrSpecTasksCreateFailed, ErrDetailInvalidRequestBody)
		return
	}

	dueDate, err := parseOptionalDateOnly(req.DueDate)
	if err != nil {
		WriteError(w, http.StatusBadRequest, ErrSpecTasksCreateFailed, errDetailInvalidDateOnly("due_date"))
		return
	}

	task, err := h.svc.CreateStandaloneTask(r.Context(), h.idGen.Generate, userID, req.Title, req.Description, dueDate, req.EstimatedMinutes, req.ActualMinutes, req.Priority, req.Status)
	if err != nil {
		status, detail := taskErrToErrResponse(err)
		WriteError(w, status, ErrSpecTasksCreateFailed, detail)
		return
	}

	WriteJSON(w, http.StatusCreated, newTaskResponse(task))
}

// CreateInProject godoc
//
//	@Summary		Create project task
//	@Description	Creates a Task under an owned Project.
//	@Tags			Tasks
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			project_id	path		string				true	"Project ID"
//	@Param			request		body		TaskCreateRequest	true	"Task create request"
//	@Success		201			{object}	TaskResponse
//	@Failure		400			{object}	ErrResponse	"Invalid request body"
//	@Failure		401			{object}	ErrResponse	"Unauthorized"
//	@Failure		404			{object}	ErrResponse	"Project not found"
//	@Failure		500			{object}	ErrResponse	"Failed to create task"
//	@Router			/projects/{project_id}/tasks [post]
func (h *TaskHandler) CreateInProject(w http.ResponseWriter, r *http.Request) {
	userID, ok := taskUserIDFromRequest(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, ErrSpecTasksCreateFailed, ErrDetailUnauthorized)
		return
	}

	var req TaskCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, ErrSpecTasksCreateFailed, ErrDetailInvalidRequestBody)
		return
	}

	dueDate, err := parseOptionalDateOnly(req.DueDate)
	if err != nil {
		WriteError(w, http.StatusBadRequest, ErrSpecTasksCreateFailed, errDetailInvalidDateOnly("due_date"))
		return
	}

	task, err := h.svc.CreateProjectTask(r.Context(), h.idGen.Generate, userID, projectIDFromRequest(r), req.Title, req.Description, dueDate, req.EstimatedMinutes, req.ActualMinutes, req.Priority, req.Status)
	if err != nil {
		status, detail := taskErrToErrResponse(err)
		WriteError(w, status, ErrSpecTasksCreateFailed, detail)
		return
	}

	WriteJSON(w, http.StatusCreated, newTaskResponse(task))
}

// Get godoc
//
//	@Summary		Get task
//	@Description	Returns an owned Task with TodoItems and TaskSchedules.
//	@Tags			Tasks
//	@Produce		json
//	@Security		BearerAuth
//	@Param			task_id	path		string	true	"Task ID"
//	@Success		200		{object}	TaskAggregateResponse
//	@Failure		401		{object}	ErrResponse	"Unauthorized"
//	@Failure		404		{object}	ErrResponse	"Task not found"
//	@Failure		500		{object}	ErrResponse	"Failed to get task"
//	@Router			/tasks/{task_id} [get]
func (h *TaskHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := taskUserIDFromRequest(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, ErrSpecTasksGetFailed, ErrDetailUnauthorized)
		return
	}

	aggregate, err := h.svc.GetTask(r.Context(), userID, taskIDFromRequest(r))
	if err != nil {
		status, detail := taskErrToErrResponse(err)
		WriteError(w, status, ErrSpecTasksGetFailed, detail)
		return
	}

	WriteJSON(w, http.StatusOK, newTaskAggregateResponse(aggregate))
}

type TaskUpdateRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	DueDate     *string `json:"due_date" format:"date"`
	DueDateSet  bool    `json:"-"`
}

func (req *TaskUpdateRequest) UnmarshalJSON(data []byte) error {
	type requestAlias TaskUpdateRequest
	var alias requestAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	dueDate, dueDateSet, err := decodeNullableDateOnly(raw, "due_date")
	if err != nil {
		return err
	}
	alias.DueDate = dueDate
	alias.DueDateSet = dueDateSet

	*req = TaskUpdateRequest(alias)
	return nil
}

// Update godoc
//
//	@Summary		Update task
//	@Description	Partially updates owned Task basic information.
//	@Tags			Tasks
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			task_id	path		string				true	"Task ID"
//	@Param			request	body		TaskUpdateRequest	true	"Task update request"
//	@Success		200		{object}	TaskResponse
//	@Failure		400		{object}	ErrResponse	"Invalid request body"
//	@Failure		401		{object}	ErrResponse	"Unauthorized"
//	@Failure		404		{object}	ErrResponse	"Task not found"
//	@Failure		500		{object}	ErrResponse	"Failed to update task"
//	@Router			/tasks/{task_id} [patch]
func (h *TaskHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := taskUserIDFromRequest(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, ErrSpecTasksUpdateFailed, ErrDetailUnauthorized)
		return
	}

	var req TaskUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, ErrSpecTasksUpdateFailed, ErrDetailInvalidRequestBody)
		return
	}

	dueDate, err := parseDateOnlyPatch(req.DueDate, req.DueDateSet)
	if err != nil {
		WriteError(w, http.StatusBadRequest, ErrSpecTasksUpdateFailed, errDetailInvalidDateOnly("due_date"))
		return
	}

	task, err := h.svc.UpdateTaskBasic(r.Context(), userID, taskIDFromRequest(r), req.Title, req.Description, dueDate)
	if err != nil {
		status, detail := taskErrToErrResponse(err)
		WriteError(w, status, ErrSpecTasksUpdateFailed, detail)
		return
	}

	WriteJSON(w, http.StatusOK, newTaskResponse(task))
}

type TaskStatusUpdateRequest struct {
	Status string `json:"status"`
}

// UpdateStatus godoc
//
//	@Summary		Update task status
//	@Description	Updates status for an owned Task.
//	@Tags			Tasks
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			task_id	path		string					true	"Task ID"
//	@Param			request	body		TaskStatusUpdateRequest	true	"Task status update request"
//	@Success		200		{object}	TaskResponse
//	@Failure		400		{object}	ErrResponse	"Invalid request body"
//	@Failure		401		{object}	ErrResponse	"Unauthorized"
//	@Failure		404		{object}	ErrResponse	"Task not found"
//	@Failure		500		{object}	ErrResponse	"Failed to update task"
//	@Router			/tasks/{task_id}/status [patch]
func (h *TaskHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	userID, ok := taskUserIDFromRequest(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, ErrSpecTasksUpdateFailed, ErrDetailUnauthorized)
		return
	}

	var req TaskStatusUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, ErrSpecTasksUpdateFailed, ErrDetailInvalidRequestBody)
		return
	}

	task, err := h.svc.UpdateTaskStatus(r.Context(), userID, taskIDFromRequest(r), req.Status)
	if err != nil {
		status, detail := taskErrToErrResponse(err)
		WriteError(w, status, ErrSpecTasksUpdateFailed, detail)
		return
	}

	WriteJSON(w, http.StatusOK, newTaskResponse(task))
}

type TaskPriorityUpdateRequest struct {
	Priority string `json:"priority"`
}

// UpdatePriority godoc
//
//	@Summary		Update task priority
//	@Description	Updates priority for an owned Task.
//	@Tags			Tasks
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			task_id	path		string						true	"Task ID"
//	@Param			request	body		TaskPriorityUpdateRequest	true	"Task priority update request"
//	@Success		200		{object}	TaskResponse
//	@Failure		400		{object}	ErrResponse	"Invalid request body"
//	@Failure		401		{object}	ErrResponse	"Unauthorized"
//	@Failure		404		{object}	ErrResponse	"Task not found"
//	@Failure		500		{object}	ErrResponse	"Failed to update task"
//	@Router			/tasks/{task_id}/priority [patch]
func (h *TaskHandler) UpdatePriority(w http.ResponseWriter, r *http.Request) {
	userID, ok := taskUserIDFromRequest(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, ErrSpecTasksUpdateFailed, ErrDetailUnauthorized)
		return
	}

	var req TaskPriorityUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, ErrSpecTasksUpdateFailed, ErrDetailInvalidRequestBody)
		return
	}

	task, err := h.svc.UpdateTaskPriority(r.Context(), userID, taskIDFromRequest(r), req.Priority)
	if err != nil {
		status, detail := taskErrToErrResponse(err)
		WriteError(w, status, ErrSpecTasksUpdateFailed, detail)
		return
	}

	WriteJSON(w, http.StatusOK, newTaskResponse(task))
}

type TaskEstimationUpdateRequest struct {
	EstimatedMinutes *int `json:"estimated_minutes"`
	ActualMinutes    *int `json:"actual_minutes"`
}

// UpdateEstimation godoc
//
//	@Summary		Update task estimation
//	@Description	Updates estimated and/or actual minutes for an owned Task.
//	@Tags			Tasks
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			task_id	path		string						true	"Task ID"
//	@Param			request	body		TaskEstimationUpdateRequest	true	"Task estimation update request"
//	@Success		200		{object}	TaskResponse
//	@Failure		400		{object}	ErrResponse	"Invalid request body"
//	@Failure		401		{object}	ErrResponse	"Unauthorized"
//	@Failure		404		{object}	ErrResponse	"Task not found"
//	@Failure		500		{object}	ErrResponse	"Failed to update task"
//	@Router			/tasks/{task_id}/estimation [patch]
func (h *TaskHandler) UpdateEstimation(w http.ResponseWriter, r *http.Request) {
	userID, ok := taskUserIDFromRequest(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, ErrSpecTasksUpdateFailed, ErrDetailUnauthorized)
		return
	}

	var req TaskEstimationUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, ErrSpecTasksUpdateFailed, ErrDetailInvalidRequestBody)
		return
	}

	task, err := h.svc.UpdateTaskEstimation(r.Context(), userID, taskIDFromRequest(r), req.EstimatedMinutes, req.ActualMinutes)
	if err != nil {
		status, detail := taskErrToErrResponse(err)
		WriteError(w, status, ErrSpecTasksUpdateFailed, detail)
		return
	}

	WriteJSON(w, http.StatusOK, newTaskResponse(task))
}

// Delete godoc
//
//	@Summary		Delete task
//	@Description	Deletes an owned Task.
//	@Tags			Tasks
//	@Produce		json
//	@Security		BearerAuth
//	@Param			task_id	path		string			true	"Task ID"
//	@Success		200		{object}	MessageResponse	"OK"
//	@Failure		401		{object}	ErrResponse		"Unauthorized"
//	@Failure		404		{object}	ErrResponse		"Task not found"
//	@Failure		500		{object}	ErrResponse		"Failed to delete task"
//	@Router			/tasks/{task_id} [delete]
func (h *TaskHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := taskUserIDFromRequest(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, ErrSpecTasksDeleteFailed, ErrDetailUnauthorized)
		return
	}

	if err := h.svc.DeleteTask(r.Context(), userID, taskIDFromRequest(r)); err != nil {
		status, detail := taskErrToErrResponse(err)
		WriteError(w, status, ErrSpecTasksDeleteFailed, detail)
		return
	}

	WriteMessage(w, http.StatusOK, "OK")
}

func taskUserIDFromRequest(r *http.Request) (domain.UserID, bool) {
	userID, ok := r.Context().Value(UserIDContextKey).(auth.UserID)
	if !ok || userID == "" {
		return "", false
	}
	return domain.UserID(userID), true
}

func taskIDFromRequest(r *http.Request) domain.TaskID {
	return domain.TaskID(chi.URLParam(r, "task_id"))
}

func todoItemIDFromRequest(r *http.Request) domain.TodoItemID {
	return domain.TodoItemID(chi.URLParam(r, "todo_item_id"))
}

func taskScheduleIDFromRequest(r *http.Request) domain.TaskScheduleID {
	return domain.TaskScheduleID(chi.URLParam(r, "task_schedule_id"))
}

func taskErrToErrResponse(err error) (int, ErrDetail) {
	switch {
	case errors.Is(err, taskusecase.ErrTaskNotFound):
		return http.StatusNotFound, NewErrDetail("task_id", "task_not_found", "Task not found")
	case errors.Is(err, taskusecase.ErrTaskProjectNotFound), errors.Is(err, domain.ErrProjectNotFound):
		return http.StatusNotFound, NewErrDetail("project_id", "project_not_found", "Project not found")
	case errors.Is(err, taskusecase.ErrTaskEstimationUpdateEmpty):
		return http.StatusBadRequest, NewErrDetail("", "empty_estimation_update", "Estimated minutes or actual minutes must be provided")
	case errors.Is(err, taskusecase.ErrTaskHasIncompleteTodoItems):
		return http.StatusBadRequest, NewErrDetail("", "task_has_incomplete_todo_items", "Cannot mark task as done while it has incomplete todo items")
	case errors.Is(err, domain.ErrTaskIDEmpty):
		return http.StatusBadRequest, NewErrDetail("task_id", "invalid_task_id", "Task ID is required")
	case errors.Is(err, domain.ErrTaskTitleEmpty):
		return http.StatusBadRequest, NewErrDetail("title", "task_title_required", "Task title is required")
	case errors.Is(err, domain.ErrTaskEstimatedMinutesInvalid):
		return http.StatusBadRequest, NewErrDetail("estimated_minutes", "invalid_estimated_minutes", "Estimated minutes must be greater than or equal to 0")
	case errors.Is(err, domain.ErrTaskActualMinutesInvalid):
		return http.StatusBadRequest, NewErrDetail("actual_minutes", "invalid_actual_minutes", "Actual minutes must be greater than or equal to 0")
	case errors.Is(err, domain.ErrTaskPriorityEmpty), errors.Is(err, domain.ErrTaskPriorityInvalid):
		return http.StatusBadRequest, NewErrDetail("priority", "invalid_task_priority", "Task priority must be one of the supported values")
	case errors.Is(err, domain.ErrTaskStatusEmpty), errors.Is(err, domain.ErrTaskStatusInvalid):
		return http.StatusBadRequest, NewErrDetail("status", "invalid_task_status", "Task status must be one of the supported values")
	default:
		return http.StatusInternalServerError, ErrDetailInternalServerError
	}
}
