package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/Najah7/task2todaytodo/internal/domain/shared"
	domain "github.com/Najah7/task2todaytodo/internal/domain/task"
)

type TaskScheduleHandler struct {
	svc   *domain.TaskScheduleService
	idGen shared.IDGenerator
}

func NewTaskScheduleHandler(svc *domain.TaskScheduleService, idGen shared.IDGenerator) *TaskScheduleHandler {
	return &TaskScheduleHandler{svc: svc, idGen: idGen}
}

type TaskScheduleCreateRequest struct {
	Title         string     `json:"title"`
	Description   string     `json:"description"`
	Location      string     `json:"location"`
	IntervalWeeks *int       `json:"interval_weeks"`
	Frequencies   []string   `json:"frequencies"`
	StartAt       time.Time  `json:"start_at"`
	EndAt         time.Time  `json:"end_at"`
	DueAt         *time.Time `json:"due_at"`
}

// Create godoc
//
//	@Summary		Create task schedule
//	@Description	Creates a TaskSchedule under an owned Task.
//	@Tags			TaskSchedules
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			task_id	path		string						true	"Task ID"
//	@Param			request	body		TaskScheduleCreateRequest	true	"TaskSchedule create request"
//	@Success		201		{object}	TaskScheduleResponse
//	@Failure		400		{object}	ErrResponse	"Invalid request body"
//	@Failure		401		{object}	ErrResponse	"Unauthorized"
//	@Failure		404		{object}	ErrResponse	"Task not found"
//	@Failure		500		{object}	ErrResponse	"Failed to create task schedule"
//	@Router			/tasks/{task_id}/schedules [post]
func (h *TaskScheduleHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := taskUserIDFromRequest(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, ErrSpecTaskSchedulesCreateFailed, ErrDetailUnauthorized)
		return
	}

	var req TaskScheduleCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, ErrSpecTaskSchedulesCreateFailed, ErrDetailInvalidRequestBody)
		return
	}

	var dueAt time.Time
	if req.DueAt != nil {
		dueAt = *req.DueAt
	}

	schedule, err := h.svc.Create(r.Context(), h.idGen.Generate, domain.TaskScheduleCreateParams{
		UserID:          userID,
		TaskID:          taskIDFromRequest(r),
		Title:           req.Title,
		Description:     req.Description,
		Location:        req.Location,
		IntervalWeeks:   req.IntervalWeeks,
		FrequencyValues: req.Frequencies,
		StartAt:         req.StartAt,
		EndAt:           req.EndAt,
		DueAt:           dueAt,
	})
	if err != nil {
		status, detail := taskScheduleErrToErrResponse(err)
		WriteError(w, status, ErrSpecTaskSchedulesCreateFailed, detail)
		return
	}

	WriteJSON(w, http.StatusCreated, newTaskScheduleResponse(schedule))
}

type TaskScheduleUpdateRequest struct {
	Title           *string    `json:"title"`
	Description     *string    `json:"description"`
	Location        *string    `json:"location"`
	IntervalWeeks   *int       `json:"interval_weeks"`
	FrequencyValues *[]string  `json:"frequencies"`
	StartAt         *time.Time `json:"start_at"`
	EndAt           *time.Time `json:"end_at"`
	DueAt           *time.Time `json:"due_at"`
}

func (req *TaskScheduleUpdateRequest) UnmarshalJSON(data []byte) error {
	type requestAlias TaskScheduleUpdateRequest
	var alias requestAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if value, ok := raw["due_at"]; ok {
		if string(value) == "null" {
			zero := time.Time{}
			alias.DueAt = &zero
		} else {
			var dueAt time.Time
			if err := json.Unmarshal(value, &dueAt); err != nil {
				return err
			}
			alias.DueAt = &dueAt
		}
	}

	*req = TaskScheduleUpdateRequest(alias)
	return nil
}

// Update godoc
//
//	@Summary		Update task schedule
//	@Description	Partially updates an owned TaskSchedule.
//	@Tags			TaskSchedules
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			task_id				path		string						true	"Task ID"
//	@Param			task_schedule_id	path		string						true	"TaskSchedule ID"
//	@Param			request				body		TaskScheduleUpdateRequest	true	"TaskSchedule update request"
//	@Success		200					{object}	TaskScheduleResponse
//	@Failure		400					{object}	ErrResponse	"Invalid request body"
//	@Failure		401					{object}	ErrResponse	"Unauthorized"
//	@Failure		404					{object}	ErrResponse	"TaskSchedule not found"
//	@Failure		500					{object}	ErrResponse	"Failed to update task schedule"
//	@Router			/tasks/{task_id}/schedules/{task_schedule_id} [patch]
func (h *TaskScheduleHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := taskUserIDFromRequest(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, ErrSpecTaskSchedulesUpdateFailed, ErrDetailUnauthorized)
		return
	}

	var req TaskScheduleUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, ErrSpecTaskSchedulesUpdateFailed, ErrDetailInvalidRequestBody)
		return
	}

	schedule, err := h.svc.Update(r.Context(), domain.TaskScheduleUpdateParams{
		UserID:          userID,
		TaskID:          taskIDFromRequest(r),
		ID:              taskScheduleIDFromRequest(r),
		Title:           req.Title,
		Description:     req.Description,
		Location:        req.Location,
		IntervalWeeks:   req.IntervalWeeks,
		FrequencyValues: req.FrequencyValues,
		StartAt:         req.StartAt,
		EndAt:           req.EndAt,
		DueAt:           req.DueAt,
	})
	if err != nil {
		status, detail := taskScheduleErrToErrResponse(err)
		WriteError(w, status, ErrSpecTaskSchedulesUpdateFailed, detail)
		return
	}

	WriteJSON(w, http.StatusOK, newTaskScheduleResponse(schedule))
}

// Delete godoc
//
//	@Summary		Delete task schedule
//	@Description	Deletes an owned TaskSchedule.
//	@Tags			TaskSchedules
//	@Produce		json
//	@Security		BearerAuth
//	@Param			task_id				path		string			true	"Task ID"
//	@Param			task_schedule_id	path		string			true	"TaskSchedule ID"
//	@Success		200					{object}	MessageResponse	"OK"
//	@Failure		401					{object}	ErrResponse		"Unauthorized"
//	@Failure		404					{object}	ErrResponse		"TaskSchedule not found"
//	@Failure		500					{object}	ErrResponse		"Failed to delete task schedule"
//	@Router			/tasks/{task_id}/schedules/{task_schedule_id} [delete]
func (h *TaskScheduleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := taskUserIDFromRequest(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, ErrSpecTaskSchedulesDeleteFailed, ErrDetailUnauthorized)
		return
	}

	if err := h.svc.Delete(r.Context(), userID, taskIDFromRequest(r), taskScheduleIDFromRequest(r)); err != nil {
		status, detail := taskScheduleErrToErrResponse(err)
		WriteError(w, status, ErrSpecTaskSchedulesDeleteFailed, detail)
		return
	}

	WriteMessage(w, http.StatusOK, "OK")
}

func taskScheduleErrToErrResponse(err error) (int, ErrDetail) {
	switch {
	case errors.Is(err, domain.ErrTaskScheduleNotFound):
		return http.StatusNotFound, NewErrDetail("task_schedule_id", "task_schedule_not_found", "Task schedule not found")
	case errors.Is(err, domain.ErrTaskScheduleTaskNotFound):
		return http.StatusNotFound, NewErrDetail("task_id", "task_not_found", "Task not found")
	case errors.Is(err, domain.ErrTaskScheduleTitleEmpty):
		return http.StatusBadRequest, NewErrDetail("title", "task_schedule_title_required", "Task schedule title is required")
	case errors.Is(err, domain.ErrTaskScheduleIntervalWeeksLess):
		return http.StatusBadRequest, NewErrDetail("interval_weeks", "invalid_interval_weeks", "Interval weeks must be greater than or equal to 0")
	case errors.Is(err, domain.ErrTaskScheduleStartAtEmpty):
		return http.StatusBadRequest, NewErrDetail("start_at", "task_schedule_start_at_required", "Task schedule start time is required")
	case errors.Is(err, domain.ErrTaskScheduleEndAtEmpty):
		return http.StatusBadRequest, NewErrDetail("end_at", "task_schedule_end_at_required", "Task schedule end time is required")
	case errors.Is(err, domain.ErrTaskScheduleEndAtMustBeAfterStartAt):
		return http.StatusBadRequest, NewErrDetail("end_at", "invalid_task_schedule_date_range", "Task schedule end time must be after start time")
	case errors.Is(err, domain.ErrTaskFrequencyInvalid), errors.Is(err, domain.ErrTaskFrequencyEmpty):
		return http.StatusBadRequest, NewErrDetail("frequencies", "invalid_task_frequency", "Frequencies must be supported weekday values")
	default:
		return http.StatusInternalServerError, ErrDetailInternalServerError
	}
}
