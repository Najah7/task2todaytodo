package rest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	scheduleusecase "github.com/Najah7/task2todaytodo/internal/application/schedule/usecase"
	"github.com/Najah7/task2todaytodo/internal/application/shared/recurrence"
	"github.com/Najah7/task2todaytodo/internal/port/rest/pagination"
	"github.com/go-chi/chi/v5"
)

var scheduleAssigneesFailure = NewFailureErrSpec("schedule_assignees", ActionList, "Failed to list schedule assignees")

type ScheduleHandler struct {
	schedules  scheduleusecase.ScheduleUseCases
	pageTokens *pagination.Codec
}

func NewScheduleHandler(schedules scheduleusecase.ScheduleUseCases, tokens *pagination.Codec) *ScheduleHandler {
	return &ScheduleHandler{schedules: schedules, pageTokens: tokens}
}

type ScheduleTagResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

type ScheduleAssigneeResponse struct {
	ID        string `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	IsOwner   bool   `json:"is_owner"`
}

type ScheduleAssigneeListResponse struct {
	Items []ScheduleAssigneeResponse `json:"items"`
}

// ListAssignees lists the owner and current Project members eligible for a blueprint.
//
//	@Summary	List schedule assignees
//	@Tags		Schedules
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path		string	true	"Blueprint ID"
//	@Success	200	{object}	ScheduleAssigneeListResponse
//	@Router		/schedule-blueprints/{id}/assignees [get]
func (h *ScheduleHandler) ListAssignees(w http.ResponseWriter, r *http.Request) {
	actor, ok := scheduleActor(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, scheduleAssigneesFailure, ErrDetailUnauthorized)
		return
	}
	blueprintID := chi.URLParam(r, "id")
	if blueprintID == "" {
		WriteError(w, http.StatusBadRequest, scheduleAssigneesFailure, scheduleField("id", "required", "Blueprint ID is required"))
		return
	}
	rows, err := h.schedules.ListAssignees.Execute(r.Context(), domain.UserID(actor), domain.ScheduleID(blueprintID))
	if err != nil {
		writeScheduleUseCaseError(w, scheduleAssigneesFailure, err)
		return
	}
	items := make([]ScheduleAssigneeResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, ScheduleAssigneeResponse{ID: row.ID, FirstName: row.FirstName, LastName: row.LastName, Email: row.Email, IsOwner: row.IsOwner})
	}
	WriteJSON(w, http.StatusOK, ScheduleAssigneeListResponse{Items: items})
}

func scheduleActor(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(UserIDContextKey).(string)
	return id, ok && id != ""
}

func decodeScheduleJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return false
	}
	var extra any
	return errors.Is(decoder.Decode(&extra), io.EOF)
}

func scheduleField(field, code, message string) ErrDetail { return NewErrDetail(field, code, message) }

func writeScheduleUseCaseError(w http.ResponseWriter, spec ErrSpec, err error) {
	status, detail := scheduleUseCaseError(err)
	WriteError(w, status, spec, detail)
}

func scheduleUseCaseError(err error) (int, ErrDetail) {
	switch {
	case errors.Is(err, scheduleusecase.ErrPermissionDenied):
		return http.StatusForbidden, NewErrDetail("", "permission_denied", "The caller lacks permission for this schedule operation")
	case errors.Is(err, domain.ErrScheduleNotFound), errors.Is(err, scheduleusecase.ErrScheduleProjectNotFound), errors.Is(err, scheduleusecase.ErrOccurrenceNotFound):
		return http.StatusNotFound, NewErrDetail("", "not_found", "Schedule resource was not found")
	case errors.Is(err, scheduleusecase.ErrOccurrenceRevisionMismatch):
		return http.StatusPreconditionFailed, NewErrDetail("If-Match", "revision_mismatch", "The resource changed; fetch the latest version and retry")
	case errors.Is(err, scheduleusecase.ErrOccurrenceInactive):
		return http.StatusConflict, scheduleField("occurrence_date", "occurrence_inactive", "Occurrence is no longer active")
	case errors.Is(err, scheduleusecase.ErrOccurrenceCompleted):
		return http.StatusConflict, scheduleField("occurrence_date", "occurrence_completed", "Completed occurrence cannot be skipped")
	case errors.Is(err, scheduleusecase.ErrOccurrenceDateRequired), errors.Is(err, scheduleusecase.ErrOccurrenceRuleMismatch), errors.Is(err, scheduleusecase.ErrBlueprintOccurrenceInvalid):
		return http.StatusBadRequest, scheduleField("occurrence_date", "invalid_occurrence", "Occurrence date does not identify an active schedule occurrence")
	case errors.Is(err, scheduleusecase.ErrRescheduleDateMismatch):
		return http.StatusBadRequest, scheduleField("start_at", "date_mismatch", "A rescheduled occurrence must keep its original local date")
	case errors.Is(err, scheduleusecase.ErrScheduleScopeInvalid):
		return http.StatusBadRequest, scheduleField("scope", "invalid_scope", "Scope must be current or following")
	case errors.Is(err, scheduleusecase.ErrScheduleAssigneeNotEligible):
		return http.StatusBadRequest, scheduleField("assignee_id", "assignee_not_eligible", "Assignee must be the owner or an eligible Project member")
	case errors.Is(err, scheduleusecase.ErrScheduleTagNotFound):
		return http.StatusNotFound, scheduleField("tag_id", "not_found", "Tag was not found")
	case errors.Is(err, scheduleusecase.ErrTimezoneNotAllowed):
		return http.StatusBadRequest, scheduleField("timezone", "not_allowed", "Timezone is not supported")
	case errors.Is(err, recurrence.ErrRecurrenceTimezoneInvalid):
		return http.StatusBadRequest, scheduleField("timezone", "invalid_timezone", "Timezone is invalid")
	case errors.Is(err, scheduleusecase.ErrTimezoneValidationUnavailable):
		return http.StatusInternalServerError, ErrDetailInternalServerError
	case errors.Is(err, domain.ErrBlueprintTitleEmpty), errors.Is(err, scheduleusecase.ErrOccurrenceTitleEmpty):
		return http.StatusBadRequest, scheduleField("title", "required", "Title is required")
	case errors.Is(err, domain.ErrBlueprintTimezoneEmpty):
		return http.StatusBadRequest, scheduleField("timezone", "required", "Timezone is required")
	case errors.Is(err, domain.ErrBlueprintFrequency), errors.Is(err, recurrence.ErrBlueprintFrequencyInvalid):
		return http.StatusBadRequest, scheduleField("frequency", "invalid_frequency", "Frequency is invalid")
	case errors.Is(err, domain.ErrBlueprintInterval):
		return http.StatusBadRequest, scheduleField("interval", "invalid_interval", "Interval must be positive")
	case errors.Is(err, domain.ErrBlueprintWeekdays):
		return http.StatusBadRequest, scheduleField("weekdays", "invalid_weekdays", "Weekdays must match the selected frequency")
	case errors.Is(err, domain.ErrBlueprintDateRange):
		return http.StatusBadRequest, scheduleField("end_date", "invalid_date_range", "End date must be on or after the first occurrence")
	case errors.Is(err, domain.ErrBlueprintTimeRange), errors.Is(err, scheduleusecase.ErrOccurrenceTimeInvalid):
		return http.StatusBadRequest, scheduleField("end_at", "invalid_time_range", "End time must be after start time")
	case errors.Is(err, scheduleusecase.ErrInvalidSchedulePage):
		return http.StatusBadRequest, scheduleField("page_size", "invalid_page_size", "Page size is invalid")
	default:
		return http.StatusInternalServerError, ErrDetailInternalServerError
	}
}
