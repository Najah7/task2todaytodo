package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	taskusecase "github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/Najah7/task2todaytodo/internal/port/rest/pagination"
)

type TaskScheduleHandler struct {
	schedules  taskusecase.TaskScheduleUseCases
	ID         shared.ID
	pageTokens *pagination.Codec
}

func NewTaskScheduleHandler(schedules taskusecase.TaskScheduleUseCases, ID shared.ID, codecs ...*pagination.Codec) *TaskScheduleHandler {
	h := &TaskScheduleHandler{schedules: schedules, ID: ID}
	if len(codecs) > 0 {
		h.pageTokens = codecs[0]
	}
	return h
}

type TaskScheduleFrequencyResponse struct {
	Value   string `json:"value"`
	Label   string `json:"label"`
	LabelJp string `json:"label_jp"`
}

type TaskScheduleResponse struct {
	ID                  string                          `json:"id"`
	TaskID              string                          `json:"task_id"`
	Title               string                          `json:"title"`
	Description         string                          `json:"description"`
	Location            string                          `json:"location"`
	StartAt             string                          `json:"start_at"`
	EndAt               string                          `json:"end_at"`
	IntervalWeeks       int                             `json:"interval_weeks"`
	Frequencies         []TaskScheduleFrequencyResponse `json:"frequencies"`
	RepeatState         string                          `json:"repeat_state"`
	FrequencyAnchorDate *string                         `json:"frequency_anchor_date,omitempty"`
	SeriesID            string                          `json:"series_id"`
	OccurrenceDate      string                          `json:"occurrence_date"`
	Timezone            string                          `json:"timezone"`
	IsException         bool                            `json:"is_exception"`
	Completed           bool                            `json:"completed"`
	CreatedAt           int64                           `json:"created_at"`
	UpdatedAt           int64                           `json:"updated_at"`
}

type TaskScheduleListResponse struct {
	Items         []TaskScheduleResponse `json:"items"`
	NextPageToken string                 `json:"next_page_token"`
}

type TaskScheduleCreateRequest struct {
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	Location      string   `json:"location"`
	StartAt       *string  `json:"start_at"`
	EndAt         *string  `json:"end_at"`
	IntervalWeeks int      `json:"interval_weeks"`
	Frequencies   []string `json:"frequencies"`
}

type TaskScheduleUpdateRequest struct {
	Scope          *string              `json:"scope"`
	OccurrenceDate string               `json:"occurrence_date"`
	Title          optionalJSON[string] `json:"title" swaggertype:"string"`
	Description    optionalJSON[string] `json:"description" swaggertype:"string"`
	Location       optionalJSON[string] `json:"location" swaggertype:"string"`
}

type TaskScheduleRescheduleRequest struct {
	Scope          string  `json:"scope"`
	OccurrenceDate string  `json:"occurrence_date"`
	StartAt        *string `json:"start_at"`
	EndAt          *string `json:"end_at"`
}

type TaskScheduleOccurrenceRequest struct {
	OccurrenceDate string `json:"occurrence_date"`
}

type TaskScheduleFrequencyRequest struct {
	IntervalWeeks *int     `json:"interval_weeks" binding:"required"`
	Frequencies   []string `json:"frequencies" binding:"required"`
}

var (
	taskScheduleListFailure      = NewFailureErrSpec(ResourceTaskSchedules, ActionList, "Failed to list task schedules")
	taskScheduleFrequencyFailure = NewFailureErrSpec(ResourceTaskSchedules, "update_frequency", "Failed to update task schedule frequency")
	taskScheduleCompleteFailure  = NewFailureErrSpec(ResourceTaskSchedules, "complete", "Failed to complete task schedule")
	taskScheduleReopenFailure    = NewFailureErrSpec(ResourceTaskSchedules, "reopen", "Failed to reopen task schedule")
	taskScheduleSkipFailure      = NewFailureErrSpec(ResourceTaskSchedules, "skip", "Failed to skip task schedule")
	taskScheduleRestoreFailure   = NewFailureErrSpec(ResourceTaskSchedules, "restore", "Failed to restore task schedule")
)

// List returns schedules belonging to the specified owned task.
//
//	@Summary	List task schedules
//	@Tags		Task Schedules
//	@Produce	json
//	@Security	BearerAuth
//	@Param		taskId		path		string	true	"Task ID"
//	@Param		page_size	query		int		false	"Items per page (default 50, maximum 100)"
//	@Param		page_token	query		string	false	"Opaque next page token"
//	@Param		from_date	query		string	false	"First occurrence date (YYYY-MM-DD); defaults to today in each series timezone"
//	@Param		fields		query		string	false	"Response field mask"
//	@Success	200			{object}	TaskScheduleListResponse
//	@Failure	400			{object}	ErrResponse
//	@Failure	401			{object}	ErrResponse
//	@Failure	404			{object}	ErrResponse
//	@Failure	500			{object}	ErrResponse
//	@Router		/tasks/{taskId}/schedules [get]
func (h *TaskScheduleHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, taskID, _, ok := taskScheduleRequestIDs(w, r, taskScheduleListFailure, false)
	if !ok {
		return
	}
	request, err := parseListRequestWithFromDate(r, h.pageTokens, string(userID), "task_schedules", string(taskID), "start_at_asc_series_id_asc_occurrence_date_asc", listEnvelope[TaskScheduleResponse]{}, true)
	if err != nil {
		writeListError(w, taskScheduleListFailure, err)
		return
	}
	var cursor *taskusecase.CursorAnchor
	if request.Anchor != nil {
		cursor = &taskusecase.CursorAnchor{At: request.Anchor.At, SeriesID: request.Anchor.SeriesID, Date: request.Anchor.OccurrenceDate, AsOf: request.Anchor.AsOf}
	}
	page, err := h.schedules.List.ExecutePage(r.Context(), userID, taskID, taskusecase.CursorPageRequest{Size: request.Size, FromDate: request.FromDate, AsOf: request.AsOf, Anchor: cursor})
	if err != nil {
		writeTaskScheduleUseCaseError(w, taskScheduleListFailure, err)
		return
	}
	items := make([]TaskScheduleResponse, 0, len(page.Items))
	anchors := make([]listAnchor, 0, len(page.Items))
	for _, schedule := range page.Items {
		items = append(items, taskScheduleResponse(schedule))
		asOf := ""
		if page.Next != nil {
			asOf = page.Next.AsOf
		}
		anchors = append(anchors, listAnchor{At: schedule.CursorStartAt, SeriesID: schedule.SeriesID, OccurrenceDate: schedule.OccurrenceDate, AsOf: asOf})
	}
	writeListResponse(w, items, anchors, page.Next != nil, request, h.pageTokens, taskScheduleListFailure)
}

// Create creates a one-off or recurring schedule for an owned task.
//
//	@Summary	Create task schedule
//	@Tags		Task Schedules
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		taskId	path		string						true	"Task ID"
//	@Param		request	body		TaskScheduleCreateRequest	true	"Task schedule"
//	@Success	201		{object}	TaskScheduleResponse
//	@Failure	400		{object}	ErrResponse
//	@Failure	401		{object}	ErrResponse
//	@Failure	404		{object}	ErrResponse
//	@Failure	409		{object}	ErrResponse
//	@Failure	500		{object}	ErrResponse
//	@Router		/tasks/{taskId}/schedules [post]
func (h *TaskScheduleHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, taskID, _, ok := taskScheduleRequestIDs(w, r, ErrSpecTaskSchedulesCreateFailed, false)
	if !ok {
		return
	}
	var request TaskScheduleCreateRequest
	if !decodeTaskScheduleJSON(w, r, &request) {
		writeTaskScheduleError(w, http.StatusBadRequest, ErrSpecTaskSchedulesCreateFailed, ErrDetailInvalidRequestBody)
		return
	}
	startAt, err := parseTaskScheduleTimestamp(request.StartAt)
	if err != nil {
		writeTaskScheduleError(w, http.StatusBadRequest, ErrSpecTaskSchedulesCreateFailed, taskScheduleInvalidField("start_at", "invalid_datetime", "Start time must be RFC3339"))
		return
	}
	endAt, err := parseTaskScheduleTimestamp(request.EndAt)
	if err != nil {
		writeTaskScheduleError(w, http.StatusBadRequest, ErrSpecTaskSchedulesCreateFailed, taskScheduleInvalidField("end_at", "invalid_datetime", "End time must be RFC3339"))
		return
	}
	schedule, err := h.schedules.Create.Execute(r.Context(), taskusecase.CreateTaskScheduleInput{
		ID: domain.TaskScheduleID(h.ID.Generate()), UserID: userID, TaskID: taskID,
		Title: request.Title, Description: request.Description, Location: request.Location,
		StartAt: startAt, EndAt: endAt, IntervalWeeks: request.IntervalWeeks, Frequencies: request.Frequencies,
	})
	if err != nil {
		writeTaskScheduleUseCaseError(w, ErrSpecTaskSchedulesCreateFailed, err)
		return
	}
	WriteJSON(w, http.StatusCreated, taskScheduleResponse(schedule))
}

// Update changes one occurrence or the root template. Start and end times use the reschedule route.
//
//	@Summary	Update task schedule details
//	@Tags		Task Schedules
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		taskId	path		string						true	"Task ID"
//	@Param		id		path		string						true	"Task schedule series ID"
//	@Param		request	body		TaskScheduleUpdateRequest	true	"Schedule fields"
//	@Success	200		{object}	TaskScheduleResponse
//	@Failure	400		{object}	ErrResponse
//	@Failure	401		{object}	ErrResponse
//	@Failure	404		{object}	ErrResponse
//	@Failure	500		{object}	ErrResponse
//	@Router		/tasks/{taskId}/schedules/{id} [patch]
func (h *TaskScheduleHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, taskID, scheduleID, ok := taskScheduleRequestIDs(w, r, ErrSpecTaskSchedulesUpdateFailed, true)
	if !ok {
		return
	}
	var request TaskScheduleUpdateRequest
	if !decodeTaskScheduleJSON(w, r, &request) {
		writeTaskScheduleError(w, http.StatusBadRequest, ErrSpecTaskSchedulesUpdateFailed, ErrDetailInvalidRequestBody)
		return
	}
	scope, validScope := taskScheduleBodyScope(request.Scope)
	if !validScope {
		writeTaskScheduleError(w, http.StatusBadRequest, ErrSpecTaskSchedulesUpdateFailed, taskScheduleInvalidField("scope", "invalid_scope", "Scope must be current or future"))
		return
	}
	if request.OccurrenceDate != "" {
		if _, err := parseDate(request.OccurrenceDate); err != nil {
			writeTaskScheduleError(w, http.StatusBadRequest, ErrSpecTaskSchedulesUpdateFailed, taskScheduleInvalidField("occurrence_date", "invalid_date", "Occurrence date must use YYYY-MM-DD format"))
			return
		}
	}
	patch := taskSchedulePatchFields(request)
	title, err := taskSchedulePatchField(patch, "title", decodeTaskScheduleString)
	if err != nil {
		writeTaskScheduleError(w, http.StatusBadRequest, ErrSpecTaskSchedulesUpdateFailed, taskScheduleInvalidField("title", "invalid_title", "Title must be a string or null"))
		return
	}
	description, err := taskSchedulePatchField(patch, "description", decodeTaskScheduleString)
	if err != nil {
		writeTaskScheduleError(w, http.StatusBadRequest, ErrSpecTaskSchedulesUpdateFailed, taskScheduleInvalidField("description", "invalid_description", "Description must be a string or null"))
		return
	}
	location, err := taskSchedulePatchField(patch, "location", decodeTaskScheduleString)
	if err != nil {
		writeTaskScheduleError(w, http.StatusBadRequest, ErrSpecTaskSchedulesUpdateFailed, taskScheduleInvalidField("location", "invalid_location", "Location must be a string or null"))
		return
	}
	var updated dao.TaskSchedule
	if scope == "future" {
		updated, err = h.schedules.Update.Execute(r.Context(), userID, taskID, scheduleID, scope, title, description, location)
	} else {
		updated, err = h.schedules.Update.ExecuteOccurrence(r.Context(), userID, taskID, scheduleID, request.OccurrenceDate, scope, title, description, location)
	}
	if err != nil {
		writeTaskScheduleUseCaseError(w, ErrSpecTaskSchedulesUpdateFailed, err)
		return
	}
	WriteJSON(w, http.StatusOK, taskScheduleResponse(updated))
}

// Delete removes the entire task schedule series, or one-off schedule.
//
//	@Summary	Delete task schedule
//	@Tags		Task Schedules
//	@Security	BearerAuth
//	@Param		taskId	path	string	true	"Task ID"
//	@Param		id		path	string	true	"Task schedule series ID"
//	@Success	204
//	@Failure	400	{object}	ErrResponse
//	@Failure	401	{object}	ErrResponse
//	@Failure	404	{object}	ErrResponse
//	@Failure	500	{object}	ErrResponse
//	@Router		/tasks/{taskId}/schedules/{id} [delete]
func (h *TaskScheduleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, taskID, scheduleID, ok := taskScheduleRequestIDs(w, r, ErrSpecTaskSchedulesDeleteFailed, true)
	if !ok {
		return
	}
	if err := h.schedules.Delete.DeleteSeries(r.Context(), userID, taskID, scheduleID); err != nil {
		writeTaskScheduleUseCaseError(w, ErrSpecTaskSchedulesDeleteFailed, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Reschedule changes start and end times for one occurrence or the root template.
//
//	@Description	Future scope updates the root template immediately and does not require occurrence_date. Current scope changes selected occurrence. occurrence_date is stable occurrence identity and may differ from start_at after an edit.
//
//	@Summary	Reschedule task schedule
//	@Tags		Task Schedules
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		taskId	path		string							true	"Task ID"
//	@Param		id		path		string							true	"Task schedule series ID"
//	@Param		request	body		TaskScheduleRescheduleRequest	true	"New schedule times"
//	@Success	200		{object}	TaskScheduleResponse
//	@Failure	400		{object}	ErrResponse
//	@Failure	401		{object}	ErrResponse
//	@Failure	404		{object}	ErrResponse
//	@Failure	500		{object}	ErrResponse
//	@Router		/tasks/{taskId}/schedules/{id}:reschedule [post]
func (h *TaskScheduleHandler) Reschedule(w http.ResponseWriter, r *http.Request) {
	userID, taskID, scheduleID, ok := taskScheduleRequestIDs(w, r, ErrSpecTaskSchedulesUpdateFailed, true)
	if !ok {
		return
	}
	var request TaskScheduleRescheduleRequest
	if !decodeTaskScheduleJSON(w, r, &request) {
		writeTaskScheduleError(w, http.StatusBadRequest, ErrSpecTaskSchedulesUpdateFailed, ErrDetailInvalidRequestBody)
		return
	}
	if request.Scope != "current" && request.Scope != "future" {
		writeTaskScheduleError(w, http.StatusBadRequest, ErrSpecTaskSchedulesUpdateFailed, taskScheduleInvalidField("scope", "invalid_scope", "Scope must be current or future"))
		return
	}
	if request.OccurrenceDate != "" {
		if _, err := parseDate(request.OccurrenceDate); err != nil {
			writeTaskScheduleError(w, http.StatusBadRequest, ErrSpecTaskSchedulesUpdateFailed, taskScheduleInvalidField("occurrence_date", "invalid_date", "Occurrence date must use YYYY-MM-DD format"))
			return
		}
	}
	startAt, err := parseTaskScheduleTimestamp(request.StartAt)
	if err != nil {
		writeTaskScheduleError(w, http.StatusBadRequest, ErrSpecTaskSchedulesUpdateFailed, taskScheduleInvalidField("start_at", "invalid_datetime", "Start time must be RFC3339"))
		return
	}
	endAt, err := parseTaskScheduleTimestamp(request.EndAt)
	if err != nil {
		writeTaskScheduleError(w, http.StatusBadRequest, ErrSpecTaskSchedulesUpdateFailed, taskScheduleInvalidField("end_at", "invalid_datetime", "End time must be RFC3339"))
		return
	}
	updated, err := h.schedules.Reschedule.Execute(r.Context(), taskusecase.RescheduleTaskScheduleInput{
		UserID: userID, TaskID: taskID, TaskScheduleID: scheduleID,
		StartAt: startAt, EndAt: endAt, Scope: request.Scope, OccurrenceDate: request.OccurrenceDate,
	})
	if err != nil {
		writeTaskScheduleUseCaseError(w, ErrSpecTaskSchedulesUpdateFailed, err)
		return
	}
	WriteJSON(w, http.StatusOK, taskScheduleResponse(updated))
}

// UpdateFrequency changes recurrence settings immediately on the root schedule. interval_weeks zero stops future generation.
//
//	@Summary	Update task schedule frequency
//	@Description	Changes recurrence settings immediately on the root schedule. frequencies selects weekdays and interval_weeks sets the number of weeks between matching weeks. Use this endpoint to change recurrence weekdays. interval_weeks zero stops future generation; occurrence_date is not part of this request.
//	@Tags		Task Schedules
//	@Accept		json
//	@Security	BearerAuth
//	@Param		taskId	path	string							true	"Task ID"
//	@Param		id		path	string							true	"Task schedule series ID"
//	@Param		request	body	TaskScheduleFrequencyRequest	true	"Recurrence settings"
//	@Success	204
//	@Failure	400	{object}	ErrResponse
//	@Failure	401	{object}	ErrResponse
//	@Failure	404	{object}	ErrResponse
//	@Failure	500	{object}	ErrResponse
//	@Router		/tasks/{taskId}/schedules/{id}/frequency [put]
func (h *TaskScheduleHandler) UpdateFrequency(w http.ResponseWriter, r *http.Request) {
	userID, taskID, scheduleID, ok := taskScheduleRequestIDs(w, r, taskScheduleFrequencyFailure, true)
	if !ok {
		return
	}
	var raw map[string]json.RawMessage
	if !decodeTaskScheduleJSON(w, r, &raw) || raw == nil || !hasTaskScheduleFrequencyFields(raw) {
		writeTaskScheduleError(w, http.StatusBadRequest, taskScheduleFrequencyFailure, ErrDetailInvalidRequestBody)
		return
	}
	var request TaskScheduleFrequencyRequest
	if err := json.Unmarshal(mustTaskScheduleJSON(raw), &request); err != nil {
		writeTaskScheduleError(w, http.StatusBadRequest, taskScheduleFrequencyFailure, ErrDetailInvalidRequestBody)
		return
	}
	if request.IntervalWeeks == nil {
		writeTaskScheduleError(w, http.StatusBadRequest, taskScheduleFrequencyFailure, taskScheduleInvalidField("interval_weeks", "required", "Interval weeks is required"))
		return
	}
	if err := h.schedules.UpdateFrequency.Execute(r.Context(), taskusecase.UpdateTaskScheduleFrequencyInput{
		UserID: userID, TaskID: taskID, TaskScheduleID: scheduleID,
		IntervalWeeks: *request.IntervalWeeks, Frequencies: request.Frequencies,
	}); err != nil {
		writeTaskScheduleUseCaseError(w, taskScheduleFrequencyFailure, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Complete marks only the selected task schedule occurrence complete.
//
//	@Summary		Complete task schedule
//	@Description	Marks one TaskSchedule occurrence complete. Recurring schedules require occurrence_date; one-off schedules may omit it. occurrence_date identifies scheduled occurrence and may differ from start_at after an edit.
//	@Tags			Task Schedules
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			taskId	path		string	true	"Task ID"
//	@Param			id		path		string	true	"Task schedule series ID"
//	@Param			request	body		TaskScheduleOccurrenceRequest	false	"Stable occurrence date; required for recurring schedules"
//	@Success		200		{object}	MessageResponse
//	@Failure		400		{object}	ErrResponse
//	@Failure		401		{object}	ErrResponse
//	@Failure		404		{object}	ErrResponse
//	@Failure		500		{object}	ErrResponse
//	@Router			/tasks/{taskId}/schedules/{id}:complete [post]
func (h *TaskScheduleHandler) Complete(w http.ResponseWriter, r *http.Request) {
	h.changeCompleted(w, r, taskScheduleCompleteFailure, h.schedules.Complete.ExecuteOccurrence, "TaskSchedule completed")
}

// Reopen clears completion for one task schedule occurrence.
//
//	@Summary		Reopen task schedule
//	@Description	Reopens one TaskSchedule occurrence. Recurring schedules require occurrence_date; one-off schedules may omit it. occurrence_date identifies scheduled occurrence and may differ from start_at after an edit.
//	@Tags			Task Schedules
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			taskId	path		string	true	"Task ID"
//	@Param			id		path		string	true	"Task schedule series ID"
//	@Param			request	body		TaskScheduleOccurrenceRequest	false	"Stable occurrence date; required for recurring schedules"
//	@Success		200		{object}	MessageResponse
//	@Failure		400		{object}	ErrResponse
//	@Failure		401		{object}	ErrResponse
//	@Failure		404		{object}	ErrResponse
//	@Failure		500		{object}	ErrResponse
//	@Router			/tasks/{taskId}/schedules/{id}:reopen [post]
func (h *TaskScheduleHandler) Reopen(w http.ResponseWriter, r *http.Request) {
	h.changeCompleted(w, r, taskScheduleReopenFailure, h.schedules.Reopen.ExecuteOccurrence, "TaskSchedule reopened")
}

// Skip marks one recurring TaskSchedule occurrence skipped. occurrence_date identifies stable occurrence, even if start_at changed.
//
//	@Summary Skip task schedule occurrence
//	@Tags Task Schedules
//	@Accept json
//	@Produce json
//	@Security BearerAuth
//	@Param taskId path string true "Task ID"
//	@Param id path string true "Task schedule series ID"
//	@Param request body TaskScheduleOccurrenceRequest true "Stable recurring occurrence date"
//	@Success 200 {object} MessageResponse
//	@Failure 400 {object} ErrResponse
//	@Failure 401 {object} ErrResponse
//	@Failure 404 {object} ErrResponse
//	@Failure 409 {object} ErrResponse "Completed occurrence cannot be skipped"
//	@Failure 500 {object} ErrResponse
//	@Router /tasks/{taskId}/schedules/{id}:skip [post]
func (h *TaskScheduleHandler) Skip(w http.ResponseWriter, r *http.Request) {
	h.changeCompleted(w, r, taskScheduleSkipFailure, h.schedules.Skip.Execute, "TaskSchedule skipped")
}

// Restore restores one skipped TaskSchedule occurrence. occurrence_date identifies stable occurrence, even if start_at changed.
//
//	@Summary Restore task schedule occurrence
//	@Tags Task Schedules
//	@Accept json
//	@Produce json
//	@Security BearerAuth
//	@Param taskId path string true "Task ID"
//	@Param id path string true "Task schedule series ID"
//	@Param request body TaskScheduleOccurrenceRequest true "Stable recurring occurrence date"
//	@Success 200 {object} MessageResponse
//	@Failure 400 {object} ErrResponse
//	@Failure 401 {object} ErrResponse
//	@Failure 404 {object} ErrResponse
//	@Failure 500 {object} ErrResponse
//	@Router /tasks/{taskId}/schedules/{id}:restore [post]
func (h *TaskScheduleHandler) Restore(w http.ResponseWriter, r *http.Request) {
	h.changeCompleted(w, r, taskScheduleRestoreFailure, h.schedules.Restore.Execute, "TaskSchedule restored")
}

func (h *TaskScheduleHandler) changeCompleted(w http.ResponseWriter, r *http.Request, spec ErrSpec, execute func(context.Context, domain.UserID, domain.TaskID, domain.TaskScheduleID, string) error, message string) {
	userID, taskID, scheduleID, ok := taskScheduleRequestIDs(w, r, spec, true)
	if !ok {
		return
	}
	occurrenceDate, err := decodeTaskScheduleOccurrenceDate(r)
	if err != nil {
		writeTaskScheduleError(w, http.StatusBadRequest, spec, ErrDetailInvalidRequestBody)
		return
	}
	if occurrenceDate != "" {
		if _, err := parseDate(occurrenceDate); err != nil {
			writeTaskScheduleError(w, http.StatusBadRequest, spec, taskScheduleInvalidField("occurrence_date", "invalid_date", "Occurrence date must use YYYY-MM-DD format"))
			return
		}
	}
	if err := execute(r.Context(), userID, taskID, scheduleID, occurrenceDate); err != nil {
		writeTaskScheduleUseCaseError(w, spec, err)
		return
	}
	WriteMessage(w, http.StatusOK, message)
}

func taskScheduleRequestIDs(w http.ResponseWriter, r *http.Request, spec ErrSpec, includeScheduleID bool) (domain.UserID, domain.TaskID, domain.TaskScheduleID, bool) {
	userID, ok := taskScheduleUserID(r.Context())
	if !ok {
		writeTaskScheduleError(w, http.StatusUnauthorized, spec, ErrDetailUnauthorized)
		return "", "", "", false
	}
	taskID := r.PathValue("taskId")
	if taskID == "" {
		writeTaskScheduleError(w, http.StatusBadRequest, spec, taskScheduleInvalidField("taskId", "required", "Task ID is required"))
		return "", "", "", false
	}
	var scheduleID domain.TaskScheduleID
	if includeScheduleID {
		value := r.PathValue("id")
		if value == "" {
			writeTaskScheduleError(w, http.StatusBadRequest, spec, taskScheduleInvalidField("id", "required", "Task schedule ID is required"))
			return "", "", "", false
		}
		scheduleID = domain.TaskScheduleID(value)
	}
	return userID, domain.TaskID(taskID), scheduleID, true
}

func taskScheduleUserID(ctx context.Context) (domain.UserID, bool) {
	id, ok := ctx.Value(UserIDContextKey).(string)
	if !ok || id == "" {
		return "", false
	}
	return domain.UserID(id), true
}

func decodeTaskScheduleJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return false
	}
	var extra any
	return errors.Is(decoder.Decode(&extra), io.EOF)
}

func decodeTaskScheduleOccurrenceDate(r *http.Request) (string, error) {
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
	var request TaskScheduleOccurrenceRequest
	if err := decoder.Decode(&request); err != nil {
		return "", err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return "", errors.New("request body must contain one JSON value")
	}
	return request.OccurrenceDate, nil
}

func taskSchedulePatchFields(request TaskScheduleUpdateRequest) TaskScheduleUpdateRequestMap {
	fields := TaskScheduleUpdateRequestMap{}
	addTaskSchedulePatchField(fields, "title", request.Title)
	addTaskSchedulePatchField(fields, "description", request.Description)
	addTaskSchedulePatchField(fields, "location", request.Location)
	return fields
}

func addTaskSchedulePatchField(fields TaskScheduleUpdateRequestMap, name string, value optionalJSON[string]) {
	if !value.Present {
		return
	}
	if value.Value == nil {
		fields[name] = json.RawMessage("null")
		return
	}
	encoded, err := json.Marshal(*value.Value)
	if err == nil {
		fields[name] = encoded
	}
}

type TaskScheduleUpdateRequestMap map[string]json.RawMessage

func taskScheduleBodyScope(scope *string) (string, bool) {
	if scope == nil || (*scope != "current" && *scope != "future") {
		return "", false
	}
	return *scope, true
}

func taskSchedulePatchField[T any](fields TaskScheduleUpdateRequestMap, field string, decode func(json.RawMessage) (T, error)) (taskusecase.PatchField[T], error) {
	raw, present := fields[field]
	if !present {
		return taskusecase.PatchField[T]{}, nil
	}
	patch := taskusecase.PatchField[T]{Present: true}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return patch, nil
	}
	value, err := decode(raw)
	if err != nil {
		return taskusecase.PatchField[T]{}, err
	}
	patch.Value = &value
	return patch, nil
}

func decodeTaskScheduleString(raw json.RawMessage) (string, error) {
	var value string
	err := json.Unmarshal(raw, &value)
	return value, err
}

func parseTaskScheduleTimestamp(value *string) (time.Time, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return time.Time{}, errors.New("timestamp is required")
	}
	return time.Parse(time.RFC3339, *value)
}

func hasTaskScheduleFrequencyFields(fields map[string]json.RawMessage) bool {
	_, hasInterval := fields["interval_weeks"]
	_, hasFrequencies := fields["frequencies"]
	if !hasInterval || !hasFrequencies || len(fields) != 2 || bytes.Equal(bytes.TrimSpace(fields["interval_weeks"]), []byte("null")) || bytes.Equal(bytes.TrimSpace(fields["frequencies"]), []byte("null")) {
		return false
	}
	return true
}

func mustTaskScheduleJSON(fields map[string]json.RawMessage) []byte {
	body, _ := json.Marshal(fields)
	return body
}

func taskScheduleResponse(schedule dao.TaskSchedule) TaskScheduleResponse {
	id := schedule.ID
	if !schedule.IsException && schedule.SeriesID != "" && schedule.ID != schedule.SeriesID {
		id = virtualOccurrenceID
	}
	frequencies := make([]TaskScheduleFrequencyResponse, 0, len(schedule.Frequencies))
	for _, frequency := range schedule.Frequencies {
		frequencies = append(frequencies, TaskScheduleFrequencyResponse{Value: frequency.Value, Label: frequency.Label, LabelJp: frequency.LabelJp})
	}
	location, err := time.LoadLocation(schedule.Timezone)
	if err != nil {
		location = time.UTC
	}
	response := TaskScheduleResponse{
		ID: id, TaskID: schedule.TaskID, Title: schedule.Title, Description: schedule.Description, Location: schedule.Location,
		StartAt: time.Unix(schedule.StartAt, 0).In(location).Format(time.RFC3339), EndAt: time.Unix(schedule.EndAt, 0).In(location).Format(time.RFC3339),
		IntervalWeeks: schedule.IntervalWeeks, Frequencies: frequencies, RepeatState: schedule.RepeatState, SeriesID: schedule.SeriesID,
		OccurrenceDate: schedule.OccurrenceDate, Timezone: schedule.Timezone, IsException: schedule.IsException, Completed: schedule.Completed,
		CreatedAt: schedule.CreatedAt, UpdatedAt: schedule.UpdatedAt,
	}
	if schedule.FrequencyAnchorDate != 0 {
		anchor := time.Unix(schedule.FrequencyAnchorDate, 0).UTC().Format("2006-01-02")
		response.FrequencyAnchorDate = &anchor
	}
	return response
}

func writeTaskScheduleUseCaseError(w http.ResponseWriter, spec ErrSpec, err error) {
	status, detail := taskScheduleUseCaseError(err)
	writeTaskScheduleError(w, status, spec, detail)
}

func taskScheduleUseCaseError(err error) (int, ErrDetail) {
	switch {
	case errors.Is(err, taskusecase.ErrPermissionDenied):
		return http.StatusForbidden, NewErrDetail("", "permission_denied", "The caller lacks permission for this task schedule operation")
	case errors.Is(err, domain.ErrTaskScheduleNotFound), errors.Is(err, taskusecase.ErrTaskNotFound):
		return http.StatusNotFound, NewErrDetail("id", "not_found", "Task or schedule was not found")
	case errors.Is(err, taskusecase.ErrOccurrenceNotFound):
		return http.StatusNotFound, taskScheduleInvalidField("occurrence_date", "occurrence_not_found", "Task schedule occurrence was not found")
	case errors.Is(err, taskusecase.ErrOccurrenceDateRequired), errors.Is(err, taskusecase.ErrOccurrenceRuleMismatch):
		return http.StatusBadRequest, taskScheduleInvalidField("occurrence_date", "invalid_occurrence", "A valid occurrence date is required")
	case errors.Is(err, taskusecase.ErrRescheduleDateMismatch):
		return http.StatusBadRequest, taskScheduleInvalidField("start_at", "date_mismatch", "Future scope must keep the occurrence's local calendar date")
	case errors.Is(err, taskusecase.ErrOccurrenceInactive):
		return http.StatusConflict, taskScheduleInvalidField("occurrence_date", "occurrence_inactive", "Occurrence is no longer active")
	case errors.Is(err, taskusecase.ErrOccurrenceCompleted):
		return http.StatusConflict, taskScheduleInvalidField("occurrence_date", "occurrence_completed", "Completed occurrence cannot be skipped")
	case IsUniqueConstraint(err, "task_schedules_pkey"):
		return http.StatusConflict, NewErrDetail("id", "task_schedule_id_conflict", "Task schedule ID already exists")
	case errors.Is(err, taskusecase.ErrTaskScheduleScopeInvalid):
		return http.StatusBadRequest, taskScheduleInvalidField("scope", "invalid_scope", "Scope must be current or future")
	case errors.Is(err, taskusecase.ErrTaskSchedulePatchRequiredFieldNull):
		return http.StatusBadRequest, taskScheduleInvalidField("title", "null_not_allowed", "Title cannot be null")
	case errors.Is(err, domain.ErrTaskScheduleIDEmpty):
		return http.StatusBadRequest, taskScheduleInvalidField("id", "required", "Task schedule ID is required")
	case errors.Is(err, domain.ErrTaskScheduleTitleEmpty):
		return http.StatusBadRequest, taskScheduleInvalidField("title", "required", "Title is required")
	case errors.Is(err, domain.ErrTaskScheduleIntervalWeeksLess), errors.Is(err, domain.ErrRecurrenceIntervalWeeksLess):
		return http.StatusBadRequest, taskScheduleInvalidField("interval_weeks", "invalid_interval", "Interval weeks must be non-negative")
	case errors.Is(err, domain.ErrTaskFrequencyInvalid):
		return http.StatusBadRequest, taskScheduleInvalidField("frequencies", "invalid_frequency", "Frequency contains an unsupported weekday")
	case errors.Is(err, domain.ErrTaskScheduleStartAtEmpty):
		return http.StatusBadRequest, taskScheduleInvalidField("start_at", "required", "Start time is required")
	case errors.Is(err, domain.ErrTaskScheduleEndAtEmpty):
		return http.StatusBadRequest, taskScheduleInvalidField("end_at", "required", "End time is required")
	case errors.Is(err, domain.ErrTaskScheduleEndAtMustBeAfterStartAt):
		return http.StatusBadRequest, taskScheduleInvalidField("end_at", "invalid_time_range", "End time must be after start time")
	default:
		return http.StatusInternalServerError, ErrDetailInternalServerError
	}
}

func taskScheduleInvalidField(field, code, message string) ErrDetail {
	return NewErrDetail(field, code, message)
}

func writeTaskScheduleError(w http.ResponseWriter, status int, spec ErrSpec, detail ErrDetail) {
	WriteError(w, status, spec, detail)
}
